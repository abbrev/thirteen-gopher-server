#!/bin/sh

# dbrowse expects 2 arguments: file and section
# $QUERY_STRING contains file and section separated by a space
exec ./dbrowse $QUERY_STRING
