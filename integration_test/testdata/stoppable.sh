#!/bin/sh
echo "Hello, I am stoppable $$"

# Close fd 3 to signal we're ready
exec 3>-

for i in {0..99}; do
    sleep 5
done
