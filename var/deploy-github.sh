#!/bin/sh
set -eu

: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"
: "${CI_DEFAULT_BRANCH:?CI_DEFAULT_BRANCH is required}"
: "${github_deploy_key:?github_deploy_key is required}"

apk add --no-cache openssh-client
mkdir -p /root/.ssh
printf '%s\n' "$github_deploy_key" > /root/.ssh/id_rsa
cat > /root/.ssh/known_hosts <<- "EOF"
|1|AuV+6vt2c6yHKSBI3cGlgiQgBw0=|oReK12ycO4x62cIfNqNIvclb2Ao= ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
|1|rLMxkb3I+R6GmInBad4kitV0ZTk=|c7GxoZTzebOPBENzRmPEylRcgtY= ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
EOF
chmod 600 /root/.ssh/id_rsa /root/.ssh/known_hosts

git remote remove github >/dev/null 2>&1 || true
git remote add github git@github.com:psono/terraform-provider-psono.git
git fetch origin "+refs/heads/${CI_DEFAULT_BRANCH}:refs/remotes/origin/${CI_DEFAULT_BRANCH}"

tag_commit="$(git rev-parse "${CI_COMMIT_TAG}^{commit}")"
branch_commit="$(git rev-parse "refs/remotes/origin/${CI_DEFAULT_BRANCH}^{commit}")"
if [ "$tag_commit" != "$branch_commit" ]; then
    echo "Release tag ${CI_COMMIT_TAG} is not the ${CI_DEFAULT_BRANCH} branch tip" >&2
    exit 1
fi

git push --atomic github \
    "refs/remotes/origin/${CI_DEFAULT_BRANCH}:refs/heads/${CI_DEFAULT_BRANCH}" \
    "refs/tags/${CI_COMMIT_TAG}:refs/tags/${CI_COMMIT_TAG}"
