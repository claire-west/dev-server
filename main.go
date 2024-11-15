/*
dev-srv serves multiple static file directories, each on its own port.

Usage:

	dev-srv [flags] [<file>]
	dev-srv [?|help|usage]
*/
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Service struct {
	dir  http.Dir
	path string
	port int
}

type RespFlagValue int

var RespFlag RespFlagValue

const (
	Invalid RespFlagValue = iota
	None
	Status
	Short
	Long
)

func flags() {
	respvals := "none|status|short|long"
	respdef := "none"
	respflagmap := map[string]RespFlagValue{
		"":       Invalid,
		"none":   None,
		"status": Status,
		"short":  Short,
		"long":   Long,
	}

	resparg := flag.String("resp", respdef, fmt.Sprintf("Logging options for the response. [%s]", respvals))

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:	%[1]s
	%[1]s [flags] [<file>]
	%[1]s [?|help|usage]

Define a list of services in a file with the following format. The default value for <file> is "./services", relative to the location of %[1]s.

	8080=/home/user/git/myfirstproject
	9090=../coolwebthing/public

Flags:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	RespFlag = respflagmap[*resparg]
	if RespFlag == Invalid {
		RespFlag = respflagmap[respdef]
		fmt.Printf("\nInvalid value for flag -resp [%s]\n", respvals)
		fmt.Printf("Continuing with default value \"%s\"\n\n", respdef)
	}
}

func main() {
	flags()
	firstarg := flag.Arg(0)
	if strings.EqualFold(firstarg, "usage") || strings.EqualFold(firstarg, "help") || firstarg == "?" {
		flag.Usage()
		return
	}

	var servicefile string
	if len(flag.Args()) == 0 || len(firstarg) == 0 {
		executable, _ := os.Executable()
		servicefile = filepath.Dir(executable) + "/services"
	} else {
		servicefile = firstarg
	}

	services := readServices(servicefile)
	start(services)
}

func start(services []Service) {
	var wg sync.WaitGroup
	wg.Add(len(services))
	wait := make(chan struct{})
	go func() {
		wg.Wait()
		wait <- struct{}{}
	}()

	servers := make(map[int]*http.Server)
	for _, service := range services {
		server := startService(service)
		servers[service.port] = server
		server.RegisterOnShutdown(func() {
			log.Printf("%s → %s\n", Yellow(service.port), Cyan("stopped"))
			delete(servers, service.port)
			wg.Done()
		})
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-wait:
		os.Exit(1)
	case <-interrupt:
		fmt.Println()
	}

	for _, server := range servers {
		go func() {
			if err := stopService(server); err != nil {
				wg.Done()
			}
		}()
	}

	select {
	case <-wait:
	case <-interrupt:
		fmt.Println()
	}
}

func readServices(servicefile string) []Service {
	read, err := os.Open(servicefile)
	if errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("%s: %s", Red("Unable to read file"), servicefile)
	}
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(read)

	var services []Service
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "=")
		port, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Fatalf("%s: %s", Red("Invalid port number"), parts[0])
		}

		services = append(services, Service{
			dir:  http.Dir(filepath.Dir(servicefile) + "/" + parts[1]),
			path: parts[1],
			port: port,
		})
	}

	read.Close()
	return services
}

type DirWithHtmlFallback struct {
	dir http.Dir
}

func (d DirWithHtmlFallback) Open(name string) (http.File, error) {
	f, err := d.dir.Open(name)
	if os.IsNotExist(err) && filepath.Ext(name) == "" {
		f, err = d.dir.Open(name + ".html")
	}
	return f, err
}

type RespWriterWithStatus struct {
	resp   http.ResponseWriter
	status int
	body   []byte
}

func WrapResponseWriter(resp http.ResponseWriter) *RespWriterWithStatus {
	return &RespWriterWithStatus{resp, 200, []byte{}}
}

func (resp *RespWriterWithStatus) Header() http.Header {
	return resp.resp.Header()
}

func (resp *RespWriterWithStatus) Write(bytes []byte) (int, error) {
	resp.body = bytes
	return resp.resp.Write(bytes)
}

func (resp *RespWriterWithStatus) WriteHeader(code int) {
	resp.status = code
	resp.resp.WriteHeader(code)
}

func LoggingHandler(next http.Handler, service Service) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		wrapped := WrapResponseWriter(resp)

		log.Printf("%s → %s %s%s", Yellow(service.port), Green(req.Method), Cyan(service.path), Blue(req.URL.Path))

		next.ServeHTTP(wrapped, req)

		if RespFlag == None {
			return
		}

		if RespFlag == Status {
			log.Printf("%s ← %s %s", Yellow(service.port), Magenta(wrapped.status), Magenta(http.StatusText(wrapped.status)))
			return
		}

		var body string
		if len(wrapped.body) > 0 {
			if RespFlag == Short {
				bytes := wrapped.body[0:int(math.Min(float64(len(wrapped.body)), 50.0))]
				body = strings.ReplaceAll(string(bytes), "\n", "")
				if len(wrapped.body) > 50 {
					body = body + "..."
				}
			} else if RespFlag == Long {
				body = "\n" + string(wrapped.body)
			}
		}

		log.Printf("%s ← %s %s %s", Yellow(service.port), Magenta(wrapped.status), Magenta(http.StatusText(wrapped.status)), body)
	})
}

func startService(service Service) *http.Server {
	dir := DirWithHtmlFallback{service.dir}
	handler := LoggingHandler(http.FileServer(dir), service)

	server := &http.Server{
		Addr: ":" + strconv.Itoa(service.port),
		Handler: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			resp.Header().Add("Access-Control-Allow-Origin", "*")
			handler.ServeHTTP(resp, req)
		}),
		ReadTimeout: time.Second * 5,
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("%s → %s %v", Yellow(service.port), Red("Error"), err)
			if err := stopService(server); err != nil {
				log.Printf("%s → %s: %v", Yellow(service.port), Red("Failed to stop server"), err)
			}
		}
	}()

	log.Printf("%s → %s\n", Yellow(service.port), Cyan(service.path))
	return server
}

func stopService(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("%s: %v", Red("Error shutting down server"), err)
		server.Close()
		return err
	}
	return nil
}
