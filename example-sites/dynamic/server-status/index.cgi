#! /bin/sh

# /server-status CGI for the Thirteen Gopher server

REQ=$((THIRTEEN_REQUESTS + 1))
BYTES=$((THIRTEEN_BYTES + 1))
KBYTES=$((THIRTEEN_BYTES / 1024 + 1))
UPTIME=$((THIRTEEN_UPTIME + 1))
REQ_PER_SEC=$(echo $REQ $UPTIME 3k / p | dc | sed 's,^\.,0.,')
BYTES_PER_SEC=$((BYTES / UPTIME))
BYTES_PER_REQ=$((BYTES / REQ))
CPU_LOAD=$(grep -o '^[0-9.]*' </proc/loadavg)
: ${CPU_LOAD:=$(uptime | sed 's/^.*averages\?: \([0-9.]*\).*$/\1/')}

unix2dos <<-EOF
	Total Accesses: $REQ
	Total kBytes: $KBYTES
	Uptime: $UPTIME
	ReqPerSec: $REQ_PER_SEC
	BytesPerSec: $BYTES_PER_SEC
	BytesPerReq: $BYTES_PER_REQ
	BusyServers: 1
	IdleServers: 0
	CPULoad: $CPU_LOAD
	Total Sessions: 0
EOF
