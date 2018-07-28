#!/bin/sh
trap 'echo "Caught and swallowed SIGTERM"' 15
echo "Hello, I am unstoppable $$"
for i in {0..99}; do
    sleep 5
done
