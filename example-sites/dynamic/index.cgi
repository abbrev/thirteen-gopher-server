#! /bin/sh


########################################################################
# Helper functions
########################################################################

export LANG=en_US.UTF-8

# $SCRIPT_DIR is the directory (under the site root) that contains the current script
export SCRIPT_DIR="${SCRIPT_NAME}"
if ! [ -d "$DOCUMENT_ROOT$SCRIPT_DIR" ]; then
	SCRIPT_DIR="${SCRIPT_DIR%/*}"
fi

condense_path() {
	echo "$1" | sed -e ':again; s,\(/\.\)\+\(/.*\|\)$,\2,g; t again' -e ':again; s,\(/[^/]*\)\?/\.\.\(/.*\|\)$,\2,g; t again'
}

# make a menu entry
# g_entry type user selector server_name server_port
g_entry() {
	local selector="$3"
	local server_name="$4"
	local server_port="$5"

	# relative selectors for this server (no .. support)
	if [ -z "$server_name" ] && echo "$selector" | grep -q -v -e '^/' -e '^[A-Za-z]\+:' -e '^$' ; then
		selector="$(condense_path "$SCRIPT_DIR/$selector")"
	fi

	printf '%c%s\t%s\t%s\t%s\r\n' \
	  "$1" "$2" "$selector" "${server_name:-$SERVER_NAME}" "${server_port:-$SERVER_PORT}"
}

# convert stdin to info lines
g_catinfo() { sed 's,^,i,; s,\r*$,\t\tnull\t70\r,'; }

# make an info line
g_info() { echo "$*" | g_catinfo; }

# make a link to a menu
g_menu() { g_entry 1 "$@"; }

# make a link to a text file
g_text() { g_entry 0 "$@"; }

# make a link to a search
g_search() { g_entry 7 "$@"; }

# make an error entry
g_error() { g_entry 3 "$*"; }

# make an error entry and exit
g_fatal() { g_error "$*"; exit 1; }

# make an error and exit if $PATH_INFO is not empty
g_error_on_path_info() { [ -n "$PATH_INFO" ] && { g_error "File not found."; exit; } }

# end of menu
g_end() { printf '.\r\n'; }

render_gph()   { "$DOCUMENT_ROOT/render-gph"; }
render_gmi()   { "$DOCUMENT_ROOT/render-gmi"; }
render_map()   { local map="$1"; if [ -x "$1" ]; then "$1"; else cat "$1"; fi | "$DOCUMENT_ROOT/render-map"; }
LYNX_OPTIONS='-width=64 -dump -dont_wrap_pre -nobrowse -with_backspaces -underscore'
render_htm()   {
	local url="-stdin"
	local postData
	if [ $# -ge 1 ]; then url="$1"; fi
	if [ $# -ge 2 ] && [ -n "$2" ]; then postData="$2"; LYNX_OPTIONS="$LYNX_OPTIONS -post_data"; fi
	printf '%s\n---\n' "$postData" | lynx $LYNX_OPTIONS -- "$url" | g_catinfo
}
render_bob()   { "$DOCUMENT_ROOT/render-bob" "$1"; }
render_rot13() { tr 'A-Za-z' 'N-ZA-Mn-za-m'; }


########################################################################
# The site's Grand Central Station
########################################################################

readable_file() { [ -f "$1" ] && [ -r "$1" ]; }

html_escape() { sed 's,&,\&amp;,g; s,",\&#34;,g; s,<,\&lt;,g; s,>,\&gt;,g; s,'\'',\&#39;,g'; }
selector_escape() { sed 's,%,%25,g; s,?,%3F,g; s,\t,%09,g; s,\r,%0D,g; s,\n,%0A,g; '; }

export SCRIPT_SELECTOR="$(printf '%s' "$SCRIPT_NAME" | selector_escape)"

if [ "$HANDLE_PATH_INFO" != n ]; then
	if echo "$SELECTOR" | grep -q 2>/dev/null '^URL:'; then
		url_html_escaped="$(echo "$SELECTOR" | sed 's,^URL:,,' | html_escape)"
		cat <<-EOF
			<html>
			<head>
			<meta http-equiv="refresh" content="10;url=$url_html_escaped">
			<title>Redirecting...</title>
			</head>
			<body>
			<p>You will be redirected to <a href="$url_html_escaped">$url_html_escaped</a> in 10 seconds.</p>
			<p>You may want to consider upgrading to a Gopher client that understands <tt>URL:</tt> selectors.</p>
			</body>
			</html>
		EOF
		exit 0
	fi

	indexes='
	index.gph
	index.gmi
	gophermap
	index.html
	index.htm
	index.dbob
	index.bob
	index.rot13
	'

	# request is for the root or there's extra path information
	if [ -z "$SCRIPT_NAME" ] || [ -n "$PATH_INFO" ]; then
		(
			path="$DOCUMENT_ROOT$SCRIPT_DIR$PATH_INFO"
			path="${path%/}" # strip trailing slash if any
			if readable_file "$path.zstd"; then
				exec zstdcat "$path.zstd"; exit 0
			elif readable_file "$path.gz"; then
				exec zcat "$path.gz"; exit 0
			elif readable_file "$path.bz2"; then
				exec bzcat "$path.bz2"; exit 0
			elif readable_file "$path.gph"; then
				# treat a gph file like a directory
				path="$path.gph"
			elif readable_file "$path.gmi"; then
				# treat a gmi file like a directory
				path="$path.gmi"
			elif readable_file "$path.html"; then
				# treat a html file like a directory
				path="$path.html"
			elif readable_file "$path.htm"; then
				# treat a htm file like a directory
				path="$path.htm"
			elif readable_file "$path.dbob"; then
				# treat a dbob file like a directory
				path="$path.dbob"
			elif readable_file "$path.bob"; then
				# treat a bob file like a directory
				path="$path.bob"
			elif readable_file "$path.rot13"; then
				# treat a rot13 file like a directory
				path="$path.rot13"
			elif ! [ -r "$path" ] || !([ -d "$path" ] || [ -f "$path" ]); then
				exit 1
			elif [ -d "$path" ]; then
				for i in $indexes; do if [ -f "$path/$i" ] && [ -r "$path/$i" ]; then path="$path/$i"; break; fi; done
			fi

			if [ -d "$path" ]; then
				# directory? list it!
				cd "$path" || exit

				LS_HEADER=y LS_PARENT=y LS_SORTBY=name LS_DETAILS=y LS_SORTREV=n "$DOCUMENT_ROOT/gopher-ls"
				exit 0
			fi

			d="${path%/*}" # the directory part
			f="${path##*/}" # the file part

			cd "$d" || exit

			# handle each index file type
			if echo "$f" | grep -q '\.gph$'; then
				render_gph <"$f"; exit 0
			elif echo "$f" | grep -q '\.gmi$'; then
				render_gmi <"$f"; exit 0
			elif [ "$f" = 'gophermap' ]; then
				render_map ./"$f"; exit 0
			elif echo "$f" | grep -q '\.html\?$'; then
				render_htm "$f"; exit 0
			elif echo "$f" | grep -q '\.dbob$'; then
				render_bob "$f" | render_gph; exit 0
			elif echo "$f" | grep -q '\.bob$'; then
				render_bob "$f"; exit 0
			elif echo "$f" | grep -q '\.rot13$'; then
				render_rot13 <"$f"; exit 0
			fi
			exit 1
		) || g_fatal "File not found."
		exit 0
	fi
fi
