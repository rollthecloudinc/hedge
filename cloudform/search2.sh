#!/bin/bash

# Variables
STACK_NAME="opensearch-cognito-stack2"
TEMPLATE_FILE="search2.yaml"  # Name of the CloudFormation template file
REGION="us-east-1"  # Change to your preferred AWS Region

# Replace these with your existing Cognito resource IDs
COGNITO_USER_POOL_ID="us-east-1_sWRV0kAgS"
COGNITO_IDENTITY_POOL_ID="us-east-1:026caf18-c852-451b-a93e-fb431c4eee6d"
MASTER_USER_ARN="arn:aws:iam::032425924121:user/tzmijewski"  # Replace with the actual master user's ARN

# Deploy the stack
echo "Deploying CloudFormation stack: $STACK_NAME"

aws cloudformation deploy \
  --template-file "$TEMPLATE_FILE" \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides "CognitoUserPoolId=$COGNITO_USER_POOL_ID" "CognitoIdentityPoolId=$COGNITO_IDENTITY_POOL_ID" "MasterUserARN=$MASTER_USER_ARN"

# Check if the stack creation/update was successful
if [ $? -eq 0 ]; then
  echo "CloudFormation stack deployed successfully!"

  # Retrieve the OpenSearch domain endpoint as an output
  OPENSEARCH_ENDPOINT=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query "Stacks[0].Outputs[?OutputKey=='OpenSearchEndpoint'].OutputValue" \
    --output text)

  echo "OpenSearch Domain Endpoint: $OPENSEARCH_ENDPOINT"
else
  echo "Failed to deploy CloudFormation stack."
  exit 1
fi