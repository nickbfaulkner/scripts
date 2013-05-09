
function gitprompt {
  dir=$(pwd)
  gitDir="$dir/.git"

  if [ ! -d $gitDir ]; then
    return
  fi

  cleanString=$(if git status | grep -q 'working directory clean'; then echo ""; else echo '*'; fi)
  branchName=$(git branch | grep '^*' | sed 's/^* //g')
  outgoingString="$(git cherry | sed -E 's/.*//g' | tr '\n' '+')"

  echo " [$cleanString$branchName$outgoingString]"
}

PS1='\w$(gitprompt) # '

