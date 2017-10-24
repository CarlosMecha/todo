#!/bin/bash

[ -z "$TODO_ADDR" ] && echo "Server address not defined" && exit 1;
[ -z "$TODO_TOKEN" ] && echo "Token not defined" && exit 1;
[ -z "$TODO_FILE" ] && echo "Todo file not defined" && exit 1;
[ -z "$TODO_EDITOR" ] && echo "Todo editor not defined" && exit 1;
[ ! -e $TODO_FILE ] && echo "Todo file doesn't exist" && exit 1;

TMP_FILE=$TODO_FILE.tmp

# Uploads the file using local date
function upload() {
    echo "Uploading file"
    local DATE=$(TZ=utc stat --print=%Y $TODO_FILE)

    curl -X PUT\
        -k\
        -H "Token: $TODO_TOKEN"\
        -H "Content-Type: plain/text"\
        -H "Last-Modified: $(date -d @$DATE +%a,\ %d\ %b\ %Y\ %H:%M:%S\ %Z)"\
        --data-binary @$TODO_FILE\
        $TODO_ADDR;

    EXIT_CODE=$?;

    if [ $EXIT_CODE -ne 0 ]; then {
        echo "Error uploading file";
    } fi;
}

# Downloads the file from the remote storage and overwrites the local file.
function download() {

    rm -rf $TMP_FILE;

    curl\
        -k\
        -o $TMP_FILE\
        -H "Token: $TODO_TOKEN"\
        -H "If-Modified-Since: $VERSION"\
        --data-binary @$TODO_FILE\
        $TODO_ADDR;

    EXIT_CODE=$?;

    if [ $EXIT_CODE -ne 0 ]; then {
        echo "Error uploading file";
        exit 1;
    } fi;

    if [ ! -e $TMP_FILE ]; then {
        echo "The file didn't get downloaded";
        exit 1;
    } fi;

}

# Retrieves the stored version.
function get_stored_version() {
    local OUTPUT=$(curl --head -k -H "Token: $TODO_TOKEN" $TODO_ADDR);

    if [ "$OUTPUT" == "" ]; then {
        echo "Error getting version file";
        exit 1;
    } fi;

    local STORED_DATE=$(echo $OUTPUT | grep Last-Modified | awk -F: '{ print $2 }')
    echo $(date -d "$STORED_DATE" +%s)
}

# Main
function main() {

    local NOW=$(date -u +%s)
    local STORED_VERSION=$(get_stored_version)
    local LOCAL_VERSION=$(TZ=utc stat --print=%Y $TODO_FILE)

    if [ $STORED_VERSION -gt $LOCAL_VERSION ]; then {
        echo "Conflict, the stored version is newer";
        exit 1;
    } fi;

    $TODO_EDITOR $TODO_FILE;

    if [ $NOW -lt $(TZ=utc stat --print=%Y $TODO_FILE) ]; then {
        upload $NEW_VERSION;
    } fi;
}

main;