#!/bin/sh
trap 'echo "Caught and swallowed SIGTERM"' 15
echo "Hello, I am unstoppable $$"

# Close fd 3 to signal we're ready
exec 3>-

for i in {0..99}; do
    sleep 5
done
