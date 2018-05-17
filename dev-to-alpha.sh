#!/bin/sh
set -e
git checkout dev
git pull --rebase
git checkout alpha
git pull --rebase
git branch -D dev-to-alpha
git checkout -b dev-to-alpha
git merge dev
git push origin dev-to-alpha
echo "============================
ADD release notes in the PR
============================"

