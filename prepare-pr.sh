#!/bin/sh

if [ -z "${2}" ]
then
  printf "%s <base> <head>

  example dev-to-alpha:
  base=dev head=alpha
" "${0}"
 exit 1
fi

BASE=$1
HEAD=$2

set -e
git checkout "${BASE}"
git pull --rebase
git checkout "${HEAD}"
git pull --rebase
set +e
git branch -D "${BASE}-to-${HEAD}"
set -e
git checkout -b "${BASE}-to-${HEAD}"
git merge "${BASE}"
git push origin "${BASE}-to-${HEAD}"

CHANGELOG="$(git log --pretty='* **%w(1000000,0,2)%b**%n  <sup>%w(10000000,0,2)%s</sup>' "${HEAD}".."${BASE}" --grep=Merge)"
MSG="$(printf "%s-to-%s\n\n%s" "${BASE}" "${HEAD}" "${CHANGELOG}")"

if [ -x "$(which hub)" ]
then
  hub pull-request -b "${HEAD}" -h "${BASE}-to-${HEAD}" -F "${MSG}" -l ready-to-test
else
  echo "============================
create a PR with base ${HEAD} and head ${BASE}-to-${HEAD} at https://github.com/zalando-incubator/kubernetes-on-aws
release notes:"
  echo "$MSG"
  echo "============================"
fi
