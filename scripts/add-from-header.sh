#!/bin/sh
# Reads message on stdin. If no From: header exists, outputs one from
# the envelope sender (passed as $1). Maddy prepends stdout to the message.
# Exit 0 = accept message.

SENDER="$1"

FOUND=0
while IFS= read -r line; do
    # Strip trailing CR
    clean=$(printf '%s' "$line" | tr -d '\r')
    # Empty line = end of headers
    [ -z "$clean" ] && break
    case "$clean" in
        [Ff][Rr][Oo][Mm]:*) FOUND=1 ;;
    esac
done

if [ "$FOUND" -eq 0 ] && [ -n "$SENDER" ]; then
    printf 'From: <%s>\r\n' "$SENDER"
fi

exit 0
