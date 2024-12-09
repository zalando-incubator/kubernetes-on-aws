#!/bin/bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <BUCKET_NAME> <NEW_ARN>"
  exit 1
fi

BUCKET_NAME="$1"
INGRESS_ROLE_ARN="$2"
STATEMENT_ID="AllowS3ReadAccess"

echo "Retrieving the current bucket policy for bucket: $BUCKET_NAME"
echo $(aws sts get-caller-identity) # TODO: remove

POLICY=$(aws s3api get-bucket-policy --bucket "$BUCKET_NAME" --query "Policy" --output text)

echo "Modifying the policy to include new ARN: $INGRESS_ROLE_ARN for statement Id: $STATEMENT_ID"
UPDATED_POLICY=$(echo "$POLICY" | jq --arg new_arn "$INGRESS_ROLE_ARN" --arg statement_id "$STATEMENT_ID" '
  .Statement |= map(
    if .Sid == $statement_id then
      if .Principal.AWS then
        .Principal.AWS |= (if type == "string" then [.] + [$new_arn] else . + [$new_arn] end)
      else
        .
      end
    else
      .
    end
  )
')

echo "Updating the bucket policy for bucket: $BUCKET_NAME"
aws s3api put-bucket-policy --bucket "$BUCKET_NAME" --policy "$UPDATED_POLICY"

echo "Bucket policy updated successfully!"
