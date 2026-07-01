package fail

fail

/*
This is a non-compiling file that has been added to explicitly ensure that CI fails.
It also contains the command that caused the failure and its output.
Remove this file if debugging locally.

./godelw verify failed after updating godel plugins and assets

Command that caused error:
./godelw exec -- go fix ./init/cli ./init/cli/time ./init/main ./integration_test ./launcher/main ./launchlib

Output:
# github.com/palantir/go-java-launcher/launchlib
# [github.com/palantir/go-java-launcher/launchlib]
fix: applied 15 of 16 fixes; 6 files updated. (Re-run the command to apply more.)
Error: exit status 1

*/
