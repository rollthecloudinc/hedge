#!/bin/bash

# Shell script to deploy an AWS CloudFormation template for OpenSearch.

# Set variables
STACK_NAME="DistinctOpenSearchStack"  # Updated the stack name to avoid conflicts
TEMPLATE_FILE="search_distinct.yaml"  # Ensure the updated YAML template file is in the same directory as this script
REGION="us-east-1"  # Change this to your target AWS region
MASTER_PASSWORD="-Password1!"  # Replace with your desired master password
COGNITO_USER_POOL_ID="us-east-1_sWRV0kAgS"  # Replace with the existing Cognito User Pool ID
COGNITO_IDENTITY_POOL_ID="us-east-1:026caf18-c852-451b-a93e-fb431c4eee6d"  # Replace with the existing Cognito Identity Pool ID

# Create the CloudFormation Stack
echo "Deploying the CloudFormation stack: ${STACK_NAME} in region: ${REGION}..."
aws cloudformation create-stack \
  --stack-name ${STACK_NAME} \
  --template-body file://${TEMPLATE_FILE} \
  --parameters \
    ParameterKey=MasterPassword,ParameterValue=${MASTER_PASSWORD} \
    ParameterKey=CognitoUserPoolId,ParameterValue=${COGNITO_USER_POOL_ID} \
    ParameterKey=CognitoIdentityPoolId,ParameterValue=${COGNITO_IDENTITY_POOL_ID} \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${REGION}

# Wait for the deployment to complete
if [ $? -eq 0 ]; then
  echo "Stack creation initiated successfully. Waiting for completion..."
  aws cloudformation wait stack-create-complete --stack-name ${STACK_NAME} --region ${REGION}
  
  if [ $? -eq 0 ]; then
    echo "Stack created successfully!"
    
    # Fetch and display the OpenSearch endpoint
    OPENSEARCH_ENDPOINT=$(aws cloudformation describe-stacks \
      --stack-name ${STACK_NAME} \
      --query "Stacks[0].Outputs[?OutputKey=='DistinctOpenSearchEndpoint'].OutputValue" \
      --output text \
      --region ${REGION})
      
    echo "OpenSearch Endpoint: ${OPENSEARCH_ENDPOINT}"
  else
    echo "Error: Stack creation failed!"
  fi
else
  echo "Error: Failed to initiate stack creation!"
fi