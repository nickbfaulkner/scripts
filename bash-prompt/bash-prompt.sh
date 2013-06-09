
function gitprompt {
  gitDir="./.git"
  gitConfig="$gitDir/config"

  if [ ! -d $gitDir ]; then
    return
  fi

  cleanString=$(if git status | grep -q 'working directory clean'; then echo ""; else echo '*'; fi)
  branchName=$(git branch | grep '^*' | sed 's/^* //g')
 
  if grep --quiet "$branchName" $gitConfig; then
    outgoingString="$(git cherry | sed -E 's/.*//g' | tr '\n' '+')"
  else
    outgoingString="_"
  fi

  echo " [$cleanString$branchName$outgoingString]"
}

PS1='\w$(gitprompt) # '

