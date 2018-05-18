#!/bin/sh
cd ~/git/teapot/kubernetes-on-aws
set -e
git checkout dev
git pull --rebase
git checkout alpha
git pull --rebase
git branch -D dev-to-alpha
git checkout -b dev-to-alpha
git merge dev
git push origin dev-to-alpha

msgBODY=$(git log --pretty=format:'%B' alpha..dev | egrep -v '^$|^Merge |^Signed-off-by |^Dev to alpha')
if [ -x $(which hub) ]
then
  hub pull-request -m "dev-to-alpha\n\n${msgBODY}" -b alpha -h dev-to-alpha
  #l ready-to-test
else
  echo "============================
create a PR with base alpha and head dev-to-alpha at https://github.com/zalando-incubator/kubernetes-on-aws
ADD release notes in the PR:
${msgBODY}
============================"
fi
