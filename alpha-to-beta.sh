#!/bin/sh
set -e
git checkout alpha
git pull --rebase
git checkout beta
git pull --rebase
git branch -D alpha-to-beta
git checkout -b alpha-to-beta
git merge alpha
git push origin alpha-to-beta
echo "============================
ADD release notes in the PR
============================"

