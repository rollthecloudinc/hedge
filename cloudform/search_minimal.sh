#!/bin/bash

# Shell script to deploy an AWS CloudFormation template for OpenSearch.

# Set variables
STACK_NAME="MinimalOpenSearchStack"
TEMPLATE_FILE="search_minimal.yaml"  # Ensure the YAML template file is in the same directory as this script
REGION="us-east-1"  # Change this to your target AWS region
MASTER_PASSWORD="-Password1!"  # Replace with your desired master password

# Create the CloudFormation Stack
echo "Deploying the CloudFormation stack: ${STACK_NAME} in region: ${REGION}..."
aws cloudformation create-stack \
  --stack-name ${STACK_NAME} \
  --template-body file://${TEMPLATE_FILE} \
  --parameters ParameterKey=MasterPassword,ParameterValue=${MASTER_PASSWORD} \
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
      --query "Stacks[0].Outputs[?OutputKey=='OpenSearchEndpoint'].OutputValue" \
      --output text \
      --region ${REGION})
      
    echo "OpenSearch Endpoint: ${OPENSEARCH_ENDPOINT}"
  else
    echo "Error: Stack creation failed!"
  fi
else
  echo "Error: Failed to initiate stack creation!"
fi