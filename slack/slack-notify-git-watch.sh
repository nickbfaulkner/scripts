#! /bin/bash

GIT_REPO=$1
SLACK_WEBHOOK=$2
GIT_DATA_DIRECTORY="${GIT_REPO}/.git/"
SECONDS_BETWEEN_CHECKS=1

echo "Watching git repo at ${GIT_REPO} and posting to Slack webhook: ${SLACK_WEBHOOK}"

if [ ! -d $GIT_DATA_DIRECTORY ]; then
  echo "Git directory found at ${GIT_DATA_DIRECTORY}"
  exit 1;
fi

function get_last_git_message() {
    echo $(git --git-dir $GIT_DATA_DIRECTORY log --pretty=oneline -1)
}

function notify_slack_of_git_change() {
    git_log=$1
    
    echo
    echo "New Git change detected..."
    echo $git_log
    echo

    slack_message=$(echo $git_log | tr '"' "'")

    curl -X POST --data-urlencode "payload={\"text\": \"$slack_message\"}" $SLACK_WEBHOOK
}

CURRENT_GIT_MESSAGE=$(get_last_git_message)
echo "Starting watching for new changes. Current version is: ${CURRENT_GIT_MESSAGE}"

while true; do
    sleep $SECONDS_BETWEEN_CHECKS
    
    NEW_GIT_MESSAGE=$(get_last_git_message)
    
    if [ "${NEW_GIT_MESSAGE}" != "${CURRENT_GIT_MESSAGE}" ]; then
        notify_slack_of_git_change "${NEW_GIT_MESSAGE}"
        CURRENT_GIT_MESSAGE="${NEW_GIT_MESSAGE}"
    fi
done
