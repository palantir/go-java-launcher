#!/bin/sh
trap 'echo "Caught and swallowed SIGTERM"' 15
echo "Hello, I am unstoppable $$"

# Close fd 3 to signal we're ready
exec 3>-

# Close &1 and &2 so sleep doesn't inherit them
exec 1>-
exec 2>-

sleep 10000
