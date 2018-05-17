git co alpha
git pull --rebase
git co beta
git pull --rebase
git br -D alpha-to-beta
git co -b alpha-to-beta
git merge alpha
git push origin alpha-to-beta
echo "============================
ADD release notes in the PR
============================"

