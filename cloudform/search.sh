#!/bin/bash

# Shell script to deploy an AWS CloudFormation template for OpenSearch.

# Set variables
STACK_NAME="BaseOpenSearchStack"
TEMPLATE_FILE="search.yaml"  # Ensure the YAML template file is in the same directory as this script
REGION="us-east-1"  # Change this to your target AWS region
MASTER_PASSWORD=""  # Replace with your desired master password
ENVIRONMENT_NAME="dev" # Replace with your environment name (e.g., dev, prod)
ENVIRONMENT_NAME_CAMEL_CASE="Dev" # Replace with the CamelCase environment name (e.g., Dev, Prod)
VENDOR_SUFFIX="rtc" # Replace with your vendor suffix
VENDOR_SUFFIX_CAMEL_CASE="Rtc" # Replace with the CamelCase vendor suffix
COGNITO_USER_POOL_ID="us-east-1_sWRV0kAgS"
COGNITO_IDENTITY_POOL_ID="us-east-1:026caf18-c852-451b-a93e-fb431c4eee6d"

# Create the CloudFormation Stack
echo "Deploying the CloudFormation stack: ${STACK_NAME} in region: ${REGION}..."
aws cloudformation create-stack \
  --stack-name ${STACK_NAME} \
  --template-body file://${TEMPLATE_FILE} \
  --parameters ParameterKey=MasterPassword,ParameterValue=${MASTER_PASSWORD} \
               ParameterKey=EnvironmentName,ParameterValue=${ENVIRONMENT_NAME} \
               ParameterKey=EnvironmentNameCamelCase,ParameterValue=${ENVIRONMENT_NAME_CAMEL_CASE} \
               ParameterKey=VendorSuffix,ParameterValue=${VENDOR_SUFFIX} \
               ParameterKey=VendorSuffixCamelCase,ParameterValue=${VENDOR_SUFFIX_CAMEL_CASE} \
               ParameterKey=UserPoolId,ParameterValue=${USER_POOL_ID} \
               ParameterKey=IdentityPoolId,ParameterValue=${IDENTITY_POOL_ID} \
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