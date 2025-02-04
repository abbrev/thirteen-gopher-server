#! /bin/bash

# A CGI to list a ZIP file's contents as a Gopher menu or to retrieve a file from inside a ZIP file.

HANDLE_PATH_INFO=n
. "$DOCUMENT_ROOT/index.cgi"

FILE="$PATH_TRANSLATED"
if ! [ -r "$FILE" ]; then
	g_error 'File not found'
	exit 1
elif ! file -L "$FILE" | grep -q 'Zip archive data'; then
	g_error 'Not a ZIP file'
	exit 1
fi

# keeping it simple: if there's a query string, retrieve that file; otherwise list ZIP contents
if [ -n "$QUERY_STRING" ]; then
	unzip -p "$FILE" "$QUERY_STRING" || g_error 'Error with ZIP file or file not found in ZIP file'
else
	# list each file in the ZIP file as a selector
	{
		# an entry looks like this:
		# "   702542  2025-02-10 16:45   hello.txt"
		#  123456789
		# (the size field may expand with files that are 1 GB or larger)
		# longest size and date/time string: "1000000000000 2025-01-01 01:01" (30)

		unzip -qql "$FILE" |
		while read -r f; do
			name="$(echo "$f" | sed 's,^ *[0-9]* *....-..-.. ..:.. *\(.*\)$,\1,')"
			gtype=9
			if false ||
				echo "$name" | grep -q '\.txt$' ||
				[ "$name" = LICENSE ]; then
				gtype=0
				# TODO add more "text" file types here
			fi
			size="$(echo "$f" | sed 's,^ *\([0-9]*\) .*$,\1,')"
			datetime="$(echo "$f" | sed 's,^ *[0-9]* *\(....-..-.. ..:..\) .*$,\1,')"
			sizedatetime="$size $datetime"
			user="$(printf '%-37.37s  %30.30s' "$name" "$datetime")"
			g_entry "$gtype" "$user" "$SCRIPT_NAME$PATH_INFO?$name"
		done
	} || g_error 'Error with ZIP file'
fi
