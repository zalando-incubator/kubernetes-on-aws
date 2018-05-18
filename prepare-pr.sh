#!/bin/sh

if [ -z $2 ]
then
  printf "$0 <base> <head>

  example dev-to-alpha:
  base=dev head=alpha
"
 exit 1
fi

BASE=$1
HEAD=$2

set -e
MSG=/tmp/pr-message
git checkout ${BASE}
git pull --rebase
git checkout ${HEAD}
git pull --rebase
set +e
git branch -D ${BASE}-to-${HEAD}
set -e
git checkout -b ${BASE}-to-${HEAD}
git merge ${BASE}
git push origin ${BASE}-to-${HEAD}

printf "${BASE}-to-${HEAD}\n"> $MSG
git log --pretty=format:'%B' ${HEAD}..${BASE} | egrep -iv '^$|^Merge |^Signed-off-by|dev to alpha|alpha to beta' | uniq >> $MSG

if [ -x $(which hub) ]
then
  hub pull-request -b ${HEAD} -h ${BASE}-to-${HEAD} -F $MSG -l ready-to-test
else
  echo "============================
create a PR with base ${HEAD} and head ${BASE}-to-${HEAD} at https://github.com/zalando-incubator/kubernetes-on-aws
release notes:"
  cat $MSG
  echo "============================"
fi
