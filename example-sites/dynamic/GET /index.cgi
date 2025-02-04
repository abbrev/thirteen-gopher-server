#! /bin/sh

# a cheeky little HTTP redirector

path="$(echo "$SELECTOR" | sed -e 's,^GET ,,; s, HTTP/1\..$,,')"
LOCATION=https://$SERVER_NAME$path

unix2dos <<-EOF
	HTTP/1.0 308 Permanent Redirect. This is a Gopher server.
	Location: $LOCATION
	Connection: close
	Content-type: text/html
	Content-length: 0

EOF
