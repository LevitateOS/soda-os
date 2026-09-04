# Show connection details after interactive login; open Setup only on consoles.
case $- in
	*i*) /usr/libexec/soda/soda-console-welcome ;;
	*) return ;;
esac

soda_console_setup() {
	[ -z "${SSH_CONNECTION-}" ] && [ -z "${SSH_TTY-}" ] || return 0
	case "$(tty 2>/dev/null)" in
		/dev/console|/dev/tty[0-9]*|/dev/ttyS[0-9]*|/dev/ttyAMA[0-9]*|/dev/hvc[0-9]*) ;;
		*) return 0 ;;
	esac
	[ "$(id -u)" -ne 0 ] || return 0
	local groups
	groups=$(id -Gn) || return 0
	case " $groups " in *" soda-workspaces "*) return 0 ;; esac
	case " $groups " in *" wheel "*) ;; *) return 0 ;; esac
	/usr/libexec/soda/soda-setup pending || return 0
	if ! sudo /usr/libexec/soda/soda-setup console; then
		printf '\nSoda Setup did not complete. Reopen it from Cockpit when needed.\n'
	fi
}
soda_console_setup
unset -f soda_console_setup
