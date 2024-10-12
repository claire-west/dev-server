# dev-server

dev-srv hosts multiple static file directories, each on its own port.

Usage:

	dev-srv
	dev-srv [<file>]

Define a set of services in a file in the following format:

	8080=/home/user/git/myfirstproject
	9090=../coolwebthing/public

The default value for <file> is "./services", relative to the location of the executable.
