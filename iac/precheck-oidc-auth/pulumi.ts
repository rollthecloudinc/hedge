import * as pulumi from "@pulumi/pulumi";
// import * as random from "@pulumi/random";
import * as aws from "@pulumi/aws";
import { getStackReference as getCognitoStackReference } from "../precheck-cognito/stack-reference";

const logger = require("./shared/logger");

// Fetch stack reference for the first project (precheck-cognito)
const cognitoStackRef = getCognitoStackReference();

// create CamelCase version of stack name as resource suffix. ie. ResourceNameDev, ResourceNameDevLocal,ect.
const resourceSuffix = pulumi.getStack()        
    .split('-') // Split by dashes
    .map(word => word.charAt(0).toUpperCase() + word.slice(1)) // Capitalize the first letter of each part
    .join(''); // Join the parts back together

// Replace config variables with stack references
const config = new pulumi.Config();
const apiDeploymentStage = config.require("ApiDeploymentStage");
const loginGovClientId = pulumi.output(cognitoStackRef.requireOutput("loginGovClientId")); // Example output
const loginGovRedirectUri = config.require("LoginGovRedirectUri"); // Example output
const loginGovIssuer = pulumi.output(cognitoStackRef.requireOutput("loginGovIssuer")); // Example output
const cognitoUserPoolId = pulumi.output(cognitoStackRef.requireOutput("userPoolId")); // Use the exported `userPoolId` from the first project
const cognitoAppClientId = pulumi.output(cognitoStackRef.requireOutput("userPoolClientId")); // Use the exported `userPoolClientId` from the first project
const cognitoRegion = pulumi.output(aws.config.region); // You can directly use the AWS region configuration

const loginGovBaseUrl = pulumi.interpolate`${loginGovIssuer}/openid_connect/authorize`;
const loginGovTokenUrl = pulumi.interpolate`${loginGovIssuer}/api/openid_connect/token`;

const resourcePrefix = 'PrecheckOidcAuth';
const resourcePrefix2 = 'precheckOidcAuth';

logger({});

const resourceName = (name: string) => resourcePrefix + name + resourceSuffix

// DynamoDB Table for Secure State and Code Verifier Storage
const stateTable = new aws.dynamodb.Table(`${resourcePrefix2}StateTable${resourceSuffix}`, {
    attributes: [{ name: "state", type: "S" }],
    hashKey: "state",
    billingMode: "PAY_PER_REQUEST",
    tags: {
        Environment: pulumi.getStack(),
    },
});

// DynamoDB Table
const authCodeTable = new aws.dynamodb.Table(`${resourcePrefix2}AuthCodeTable${resourceSuffix}`, {
    attributes: [
        { name: "auth_code", type: "S" }, // Partition key (string type)
    ],
    hashKey: "auth_code", // Specify the partition key
    billingMode: "PAY_PER_REQUEST", // On-demand pricing
    ttl: {
        attributeName: "expiration", // Set TTL field
        enabled: true,
    },
    tags: {
        Environment: pulumi.getStack(),
    },
});

// DynamoDB Table for storing redirect URLs associated with the state
const redirectTable = new aws.dynamodb.Table(`${resourcePrefix2}RedirectTable${resourceSuffix}`, {
    attributes: [
        { name: "state", type: "S" }, // 'state' is the partition key
    ],
    hashKey: "state", // Use 'state' as the primary key
    billingMode: "PAY_PER_REQUEST", // On-demand pricing mode
    ttl: {
        attributeName: "ttl", // Attribute for Time-to-Live
        enabled: true, // Enable TTL for automatic expiration of items
    },
    tags: {
        Environment: pulumi.getStack(), // Tag the table with the current stack name
    },
});

// Lambda Role for all Lambda functions
const lambdaRole = new aws.iam.Role(resourceName(`LambdaRole`), {
    assumeRolePolicy: aws.iam.assumeRolePolicyForPrincipal({ Service: "lambda.amazonaws.com" }),
});

new aws.iam.RolePolicyAttachment(resourceName(`LambdaBasicExecution`), {
    role: lambdaRole.name,
    policyArn: aws.iam.ManagedPolicies.AWSLambdaBasicExecutionRole,
});

new aws.iam.RolePolicyAttachment(resourceName(`LambdaDynamoDBAccess`), {
    role: lambdaRole.name,
    policyArn: aws.iam.ManagedPolicies.AmazonDynamoDBFullAccess, // Limit this to specific resources in production
});

// Add CloudWatch Logs permissions to the Lambda Role
new aws.iam.RolePolicyAttachment(resourceName(`LambdaCloudWatchLogs`), {
    role: lambdaRole.name,
    policyArn: aws.iam.ManagedPolicies.CloudWatchLogsFullAccess,
});

// Token Exchange Lambda Function for `/token`
const tokenExchangeLambda = new aws.lambda.Function(resourceName(`TokenExchangeLambda`), {
    runtime: aws.lambda.Runtime.NodeJS20dX,
    role: lambdaRole.arn,
    handler: "index.handler",
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./token"), // Lambda code directory for `/token` logic
        // Include the shared logger file
        "logger": new pulumi.asset.FileAsset("./shared/logger.js")
    }),
    environment: {
        variables: {
            COGNITO_APP_CLIENT_ID: cognitoAppClientId,
            STATE_TABLE_NAME: stateTable.name,
            AUTH_CODE_TABLE_NAME: authCodeTable.name,
            REDIRECT_TABLE_NAME: redirectTable.name
        },
    },
});

// Login Lambda Function for `/login`
const loginLambda = new aws.lambda.Function(resourceName("LoginLambda"), {
    runtime: aws.lambda.Runtime.NodeJS20dX,
    role: lambdaRole.arn,
    handler: "index.handler",
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./login"), // Lambda code directory for `/login` logic
        // Include the shared logger file
        "logger": new pulumi.asset.FileAsset("./shared/logger.js")
    }),
    environment: {
        variables: {
            LOGIN_GOV_CLIENT_ID: loginGovClientId,
            LOGIN_GOV_REDIRECT_URI: loginGovRedirectUri,
            LOGIN_GOV_BASE_URL: loginGovBaseUrl,
            LOGIN_GOV_TOKEN_URL: loginGovTokenUrl,
            STATE_TABLE_NAME: stateTable.name,
            REDIRECT_TABLE_NAME: redirectTable.name
        },
    },
});

// callback Lambda Function for `/callback`
const callbackLambda = new aws.lambda.Function(resourceName("CallbackLambda"), {
    runtime: aws.lambda.Runtime.NodeJS20dX,
    role: lambdaRole.arn,
    handler: "index.handler",
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./callback"), // Lambda code directory for `/callback` logic
        // Include the shared logger file
        "logger": new pulumi.asset.FileAsset("./shared/logger.js")
    }),
    environment: {
        variables: {
            LOGIN_GOV_CLIENT_ID: loginGovClientId,
            LOGIN_GOV_REDIRECT_URI: loginGovRedirectUri,
            LOGIN_GOV_TOKEN_URL: loginGovTokenUrl,
            LOGIN_GOV_ISSUER: loginGovIssuer,
            COGNITO_USER_POOL_ID: cognitoUserPoolId,
            COGNITO_APP_CLIENT_ID: cognitoAppClientId,
            COGNITO_REGION: cognitoRegion,
            STATE_TABLE_NAME: stateTable.name,
            AUTH_CODE_TABLE_NAME: authCodeTable.name,
            REDIRECT_TABLE_NAME: redirectTable.name
        },
    },
});

// User Lambda Function for `/user`
const userLambda = new aws.lambda.Function(resourceName("UserLambda"), {
    runtime: aws.lambda.Runtime.NodeJS20dX,
    role: lambdaRole.arn,
    handler: "index.handler",
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./user"), // Lambda code directory for `/user` logic
        // Include the shared logger file
        "logger": new pulumi.asset.FileAsset("./shared/logger.js")
    }),
    environment: {
        variables: {
            COGNITO_USER_POOL_ID: cognitoUserPoolId,
            COGNITO_REGION: cognitoRegion,
        },
    },
});

// Create the API Gateway
const restApi = new aws.apigateway.RestApi(resourceName("OidcAuthApiGateway"), {
    description: "API Gateway for our custom OIDC provider",
});

// Create the `/login` resource and method
const loginResource = new aws.apigateway.Resource(resourceName("LoginResource"), {
    parentId: restApi.rootResourceId,
    pathPart: "login",
    restApi: restApi.id,
});

const loginMethod = new aws.apigateway.Method(resourceName("LoginMethod"), {
    restApi: restApi.id,
    resourceId: loginResource.id,
    httpMethod: "GET",
    authorization: "NONE",
});

const loginIntegration = new aws.apigateway.Integration(resourceName("LoginIntegration"), {
    restApi: restApi.id,
    resourceId: loginResource.id,
    httpMethod: loginMethod.httpMethod,
    type: "AWS_PROXY",
    integrationHttpMethod: "POST",
    uri: loginLambda.invokeArn,
});

// Grant API Gateway permission to invoke the `/login` Lambda
new aws.lambda.Permission(resourceName("LoginPermission"), {
    action: "lambda:InvokeFunction",
    function: loginLambda.name,
    principal: "apigateway.amazonaws.com",
    sourceArn: pulumi.interpolate`${restApi.executionArn}/*/GET/login`,
});

// Create the `/callback` resource and method
const callbackResource = new aws.apigateway.Resource(resourceName("CallbackResource"), {
    parentId: restApi.rootResourceId,
    pathPart: "callback",
    restApi: restApi.id,
});

const callbackMethod = new aws.apigateway.Method(resourceName("CallbackMethod"), {
    restApi: restApi.id,
    resourceId: callbackResource.id,
    httpMethod: "GET",
    authorization: "NONE",
});

const callbackIntegration = new aws.apigateway.Integration(resourceName("CallbackIntegration"), {
    restApi: restApi.id,
    resourceId: callbackResource.id,
    httpMethod: callbackMethod.httpMethod,
    type: "AWS_PROXY",
    integrationHttpMethod: "POST",
    uri: callbackLambda.invokeArn,
});

// Grant API Gateway permission to invoke the `/callback` Lambda
new aws.lambda.Permission(resourceName("CallbackPermission"), {
    action: "lambda:InvokeFunction",
    function: callbackLambda.name,
    principal: "apigateway.amazonaws.com",
    sourceArn: pulumi.interpolate`${restApi.executionArn}/*/GET/callback`,
});

// Create the `/token` resource and method
const tokenResource = new aws.apigateway.Resource(resourceName("TokenResource"), {
    parentId: restApi.rootResourceId,
    pathPart: "token",
    restApi: restApi.id,
});

const tokenMethod = new aws.apigateway.Method(resourceName("TokenMethod"), {
    restApi: restApi.id,
    resourceId: tokenResource.id,
    httpMethod: "POST",
    authorization: "NONE"
});

const tokenIntegration = new aws.apigateway.Integration(resourceName("TokenIntegration"), {
    restApi: restApi.id,
    resourceId: tokenResource.id,
    httpMethod: tokenMethod.httpMethod,
    type: "AWS_PROXY",
    integrationHttpMethod: "POST",
    uri: tokenExchangeLambda.invokeArn,
});

// Grant API Gateway permission to invoke the `/token` Lambda
new aws.lambda.Permission(resourceName("TokenPermission"), {
    action: "lambda:InvokeFunction",
    function: tokenExchangeLambda.name,
    principal: "apigateway.amazonaws.com",
    sourceArn: pulumi.interpolate`${restApi.executionArn}/*/POST/token`,
});

// Create the `/user` resource and method
const userResource = new aws.apigateway.Resource(resourceName("UserResource"), {
    parentId: restApi.rootResourceId,
    pathPart: "user",
    restApi: restApi.id,
});

const userMethod = new aws.apigateway.Method(resourceName("UserMethod"), {
    restApi: restApi.id,
    resourceId: userResource.id,
    httpMethod: "GET",
    authorization: "NONE",
});

const userIntegration = new aws.apigateway.Integration(resourceName("UserIntegration"), {
    restApi: restApi.id,
    resourceId: userResource.id,
    httpMethod: userMethod.httpMethod,
    type: "AWS_PROXY",
    integrationHttpMethod: "POST",
    uri: userLambda.invokeArn,
});

// Grant API Gateway permission to invoke the `/user` Lambda
new aws.lambda.Permission(resourceName("UserPermission"), {
    action: "lambda:InvokeFunction",
    function: userLambda.name,
    principal: "apigateway.amazonaws.com",
    sourceArn: pulumi.interpolate`${restApi.executionArn}/*/GET/user`,
});


// Enable CloudWatch logging for API Gateway
const logGroup = new aws.cloudwatch.LogGroup(resourceName("ApiGatewayLogGroup"), {
    retentionInDays: 3, // Adjust retention period as needed
});

const apiGatewayAccount = new aws.apigateway.Account(resourceName("ApiGatewayAccount"), {
    cloudwatchRoleArn: new aws.iam.Role(resourceName("ApiGatewayCloudWatchRole"), {
        assumeRolePolicy: JSON.stringify({
            Version: "2012-10-17",
            Statement: [
                {
                    Action: "sts:AssumeRole",
                    Effect: "Allow",
                    Principal: {
                        Service: "apigateway.amazonaws.com",
                    },
                },
            ],
        }),
        managedPolicyArns: [aws.iam.ManagedPolicies.CloudWatchFullAccessV2],
    }).arn,
});

// Generate a random string for each deployment to ensure it always creates a new one
/*const deploymentUniqueString = new random.RandomString("deploymentUniqueString", {
    length: 8,
    special: false,
    upper: false,
});*/

// Create a new deployment each time based on changes to the API configuration
const apiDeployment = new aws.apigateway.Deployment(resourceName("ApiDeployment"), {
    restApi: restApi.id,
    description: `Deployment for stage ${apiDeploymentStage} - ${new Date().toISOString()}`,
}, { 
    dependsOn: [
        loginIntegration, 
        callbackIntegration, 
        userIntegration,
        tokenIntegration
    ], // Ensure deployment waits for all integrations to be ready
    replaceOnChanges: ["description"], // Force replacement whenever the description changes
});

// Deploy the API Gateway with logging enabled
const apiStage = new aws.apigateway.Stage(resourceName("ApiStage"), {
    restApi: restApi.id,
    stageName: apiDeploymentStage,
    deployment: apiDeployment.id, // Reference the new deployment
    accessLogSettings: {
        destinationArn: logGroup.arn,
        format: JSON.stringify({
            requestId: "$context.requestId",
            ip: "$context.identity.sourceIp",
            caller: "$context.identity.caller",
            user: "$context.identity.user",
            requestTime: "$context.requestTime",
            httpMethod: "$context.httpMethod",
            resourcePath: "$context.resourcePath",
            // queryString: "$context.requestQueryString",
            status: "$context.status",
            protocol: "$context.protocol",
            responseLength: "$context.responseLength",
            // requestPayload: "$context.requestOverride.body", 
            // authorization: "$context.requestOverride.header.Authorization",
            integrationErrorMessage: "$context.integrationErrorMessage",
            integrationStatus: "$context.integrationStatus"
        }),
    },
    xrayTracingEnabled: false, // Enable X-Ray tracing if needed
}, { dependsOn: [apiDeployment] });

const apiMethodSettings = new aws.apigateway.MethodSettings(resourceName("ApiMethodSettings"), {
    restApi: restApi.id,
    stageName: apiStage.stageName, // Stage name from your apiStage
    methodPath: "*/*", // Apply to all resources
    settings: {
        loggingLevel: "INFO", // Set to "INFO" or "ERROR"
        dataTraceEnabled: true, // Enable detailed request/response logging
        metricsEnabled: false, // Enable CloudWatch metrics
        throttlingBurstLimit: 1000, // Optional: Adjust throttling burst limit
        throttlingRateLimit: 500,   // Optional: Adjust throttling rate limit
    },
}, { dependsOn: [ apiStage ] });

// Export the API Gateway URL
export const apiUrl = pulumi.interpolate`${apiStage.invokeUrl}`;
export const apiGatewayStage = pulumi.interpolate`${apiStage.stageName}`;
export const apiGatewayDomain = pulumi.interpolate`${apiDeployment.invokeUrl}`;