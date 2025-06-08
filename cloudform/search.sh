#!/bin/bash

# Define variables
STACK_NAME="BaseSearchSystemStack" # Change this to your desired stack name
TEMPLATE_FILE="search.yaml" # The name of the CloudFormation template file
ENVIRONMENT_NAME="dev" # Replace with your environment name (e.g., dev, prod)
ENVIRONMENT_NAME_CAMEL_CASE="Dev" # Replace with the CamelCase environment name (e.g., Dev, Prod)
VENDOR_SUFFIX="rtc" # Replace with your vendor suffix
VENDOR_SUFFIX_CAMEL_CASE="Rtc" # Replace with the CamelCase vendor suffix
USER_POOL_ID="us-east-1_sWRV0kAgS" # Replace with your Cognito User Pool ID
IDENTITY_POOL_ID="us-east-1:026caf18-c852-451b-a93e-fb431c4eee6d" # Replace with your Cognito Identity Pool ID
REGION="us-east-1"

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "AWS CLI not found. Please install AWS CLI and configure it."
    exit 1
fi



# Deploy the CloudFormation stack
echo "Deploying CloudFormation stack..."
aws cloudformation deploy \
    --stack-name "$STACK_NAME" \
    --template-file "$TEMPLATE_FILE" \
    --parameter-overrides \
        EnvironmentName="$ENVIRONMENT_NAME" \
        EnvironmentNameCamelCase="$ENVIRONMENT_NAME_CAMEL_CASE" \
        VendorSuffix="$VENDOR_SUFFIX" \
        VendorSuffixCamelCase="$VENDOR_SUFFIX_CAMEL_CASE" \
        UserPoolId="$USER_POOL_ID" \
        IdentityPoolId="$IDENTITY_POOL_ID" \
    --capabilities CAPABILITY_NAMED_IAM CAPABILITY_AUTO_EXPAND \
    --region "$REGION"

# Check the deployment status
if [ $? -eq 0 ]; then
    echo "CloudFormation stack deployed successfully!"
else
    echo "CloudFormation stack deployment failed."
    exit 1
fi