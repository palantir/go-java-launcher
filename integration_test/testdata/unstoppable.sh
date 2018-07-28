#!/bin/sh
trap 'echo "Caught and swallowed SIGTERM"' 15
echo "Hello, I am unstoppable $$"

# Close fd 3 to signal we're ready
exec 3>-

# Close >&1 and >&2 so sleep doesn't inherit them
sleep 10000 >/dev/null 2>&1
