git co dev
git pull --rebase
git co alpha
git pull --rebase
git br -D dev-to-alpha
git co -b dev-to-alpha
git merge dev
git push origin dev-to-alpha
echo "============================
ADD release notes in the PR
============================"

