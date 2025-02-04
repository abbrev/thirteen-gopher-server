#! /bin/bash

. "$DOCUMENT_ROOT/index.cgi"

if echo "$QUERY_STRING" | grep -q '^[1-9][0-9]*$'; then
	year="$QUERY_STRING"
elif echo "$SEARCHREQUEST" | grep -q '^[1-9][0-9]*$'; then
	year="$SEARCHREQUEST"
else
	year=$(date +%Y)
fi

pyear=$((year-1))
nyear=$((year+1))

cal -y $year | g_catinfo
g_info
g_entry 1 'Previous year' "$SCRIPT_SELECTOR?$pyear"
g_entry 1 'Next year'     "$SCRIPT_SELECTOR?$nyear"
g_entry 1 'Current year'  "$SCRIPT_SELECTOR"
g_entry 7 'Jump to year'  "$SCRIPT_SELECTOR"
