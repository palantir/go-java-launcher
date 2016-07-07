#!/bin/bash

set -eu -o pipefail

# Quoting from the golint readme:
#
#     The suggestions made by golint are exactly that: suggestions. Golint is
#     not perfect, and has both false positives and false negatives. Do not
#     treat its output as a gold standard. We will not be adding pragmas or
#     other knobs to suppress specific warnings, so do not expect or require
#     code to be completely "lint-free". In short, this tool is not, and will
#     never be, trustworthy enough for its suggestions to be enforced
#     automatically, for example as part of a build process.
#
# We ignore the following suggestions. This list also exists in .codeclimate.yml
# and they should be kept in sync.
golint ./... | grep -v '^vendor\/' | { ! grep -v \
'and that stutters\|'\
'const \w\+ should be\|'\
'error should be the last type when returning multiple items\|'\
'func \w\+ should be\|'\
'func parameter \w\+ should be\|'\
'interface method parameter \w\+ should be\|'\
'method \w\+ should be\|'\
'method parameter \w\+ should be\|'\
'method result \w\+ should be\|'\
'or a comment on this block\|'\
'outdent its block\|'\
'range var \w\+ should be\|'\
'should be of the form\|'\
'should have comment or be unexported\|'\
'should not end with punctuation\|'\
'should not have leading space\|'\
'should not use dot imports\|'\
'struct field \w\+ should be\|'\
'type \w\+ should be\|'\
'var \w\+ should be'; }
