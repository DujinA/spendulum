#!/usr/bin/env bash
# Usage: deploy-prod.sh <full commit sha> <sha256:image digest>
set -euo pipefail

sha=$1
digest=$2
: "${DEPLOY_EMAIL:?}"
kustomize=${KUSTOMIZE:-go tool kustomize}
image=ghcr.io/dujina/spendulum
kustomization=deploy/overlays/prod/kustomization.yaml

# Each attempt starts again from the latest master, so a push race costs only a retry.
for attempt in 1 2 3 4 5; do
  git fetch --quiet origin master
  git reset --quiet --hard origin/master

  deployed=$(sed -n 's/^[[:space:]]*newTag: sha-\([0-9a-f]\{40\}\)$/\1/p' "$kustomization")
  if [[ -n $deployed ]] && git merge-base --is-ancestor "$sha" "$deployed" 2>/dev/null; then
    echo "master already deploys ${deployed:0:7}, which includes ${sha:0:7}; nothing to do"
    exit 0
  fi

  (cd "$(dirname "$kustomization")" && $kustomize edit set image "$image:sha-$sha@$digest")
  git -c user.name=spendulum-deploy -c user.email="$DEPLOY_EMAIL" \
    commit --quiet --message "deploy(prod): ${sha:0:7}" -- "$kustomization"

  if git push --quiet origin HEAD:master; then
    exit 0
  fi

  echo "push attempt $attempt was rejected; retrying on the latest master" >&2
  sleep $((attempt * 2))
done

echo "gave up after $attempt push attempts" >&2
exit 1
