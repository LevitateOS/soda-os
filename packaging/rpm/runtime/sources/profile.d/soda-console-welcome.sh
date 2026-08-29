# Show the dashboard connection details after interactive local or SSH logins.
case $- in
	*i*) /usr/libexec/soda/soda-console-welcome ;;
esac
