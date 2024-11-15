package main

import "fmt"

const X = "\033[0m"

func Red(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[31m", v, X)
}

func Green(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[32m", v, X)
}

func Yellow(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[33m", v, X)
}

func Blue(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[34m", v, X)
}

func Magenta(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[35m", v, X)
}

func Cyan(v any) string {
	return fmt.Sprintf("%s%v%s", "\033[36m", v, X)
}
