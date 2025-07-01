#!/bin/bash

# Variables
STACK_NAME="CognitoAuthenticatedRoleStack"
TEMPLATE_FILE="cognito_role.yaml" # Ensure the YAML template file is in the same directory
REGION="us-east-1"  # Change to your AWS target region
IDENTITY_POOL_ID="us-east-1:026caf18-c852-451b-a93e-fb431c4eee6d" # Replace with your Cognito Identity Pool ID
OPENSEARCH_DOMAIN_NAME="rtc-classifieds-dev-rtc" # Replace with your OpenSearch domain name
ENVIRONMENT_NAME_CAMEL_CASE="Dev"  # Replace with your environment name in camel case
VENDOR_SUFFIX_CAMEL_CASE="Rtc"  # Replace with your vendor suffix in camel case

# Deploy the CloudFormation stack
echo "Deploying the CloudFormation stack: ${STACK_NAME} in region: ${REGION}..."
aws cloudformation create-stack \
  --stack-name ${STACK_NAME} \
  --template-body file://${TEMPLATE_FILE} \
  --parameters ParameterKey=IdentityPoolId,ParameterValue=${IDENTITY_POOL_ID} \
               ParameterKey=OpenSearchDomainName,ParameterValue=${OPENSEARCH_DOMAIN_NAME} \
               ParameterKey=EnvironmentNameCamelCase,ParameterValue=${ENVIRONMENT_NAME_CAMEL_CASE} \
               ParameterKey=VendorSuffixCamelCase,ParameterValue=${VENDOR_SUFFIX_CAMEL_CASE} \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${REGION}

# Wait for stack creation to complete
if [ $? -eq 0 ]; then
  echo "Stack creation initiated successfully. Waiting for completion..."
  aws cloudformation wait stack-create-complete --stack-name ${STACK_NAME} --region ${REGION}
  
  if [ $? -eq 0 ]; then
    echo "Stack created successfully!"
    
    # Fetch and display the role ARN
    COGNITO_ROLE_ARN=$(aws cloudformation describe-stacks \
      --stack-name ${STACK_NAME} \
      --query "Stacks[0].Outputs[?OutputKey=='CognitoRoleArn'].OutputValue" \
      --output text \
      --region ${REGION})
      
    echo "Cognito Authenticated Role ARN: ${COGNITO_ROLE_ARN}"
  else
    echo "Error: Stack creation failed!"
  fi
else
  echo "Error: Failed to initiate stack creation!"
fi