#some fairly straightforward library routines for sending gopher traffic
#(c)2001, 2004 cameron kaiser

sub return_error {
	local($resource, $mesg, $item_type) = (@_);
	if ($item_type eq '1') {
		print STDOUT "0'$resource' $mesg\t\terror.host\t1\r\n.\r\n";
	} else {
		print STDOUT <<"EOF";
There was an error in the application $resource.
----------------------------------------------------------------------------
$resource $mesg
.
EOF
	}
	exit;
}

# common exit pathway. try offer_file for a nicer interface.
sub offer {
	# $name = display string
	# $resc = selector
	# $extent = optional trailing data (gopher plus? etc.)

	local($type, $name, $resc, $server, $port, $extent) = (@_);
	print STDOUT "$type$name\t$resc\t$server\t$port" .
		(($extent) ? "\t$extent" : "") . "\r\n";
}

sub offer_file {
	# $resc can be a relative path
	# $server and $port are optional

	local($type, $name, $resc, $server, $port, $extent) = (@_);
	local($rdir) = $ENV{'SELECTOR'};
	$rdir =~ s#/[^/]+$#/#;
	$server ||= $ENV{'SERVER_HOST'};
	$port ||= $ENV{'SERVER_PORT'};
	$resc = "$rdir$resc" if ($resc !~ m#^/#);
	&offer($type, $name, $resc, $server, $port, $extent);
}

sub offer_choice {
	# used for passing arguments to a mole virtual directory
	# somewhat exotic
	local($type, $name, $choice, $server, $port, $extent) = (@_);
	&offer_file($type, $name, "$ENV{'SELECTOR'}" . 
		(($ENV{'SELECTOR'} =~ /\?/) ? " " : "?") . "$choice",
		$server, $port, $extent);
}

sub print_string {
	# displays a string (you must be within a gopher menu)
	&offer_file('i', "@_", '', 'error.host', '1');
}

sub hr {
&print_string();
&print_string('------------------------------------------------------------');
&print_string();
}

1;

