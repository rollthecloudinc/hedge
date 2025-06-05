import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

// create CamelCase version of stack name as resource suffix. ie. ResourceNameDev, ResourceNameDevLocal,ect.
const resourceSuffix = pulumi.getStack()        
    .split('-') // Split by dashes
    .map(word => word.charAt(0).toUpperCase() + word.slice(1)) // Capitalize the first letter of each part
    .join(''); // Join the parts back together

// Configurations to replace CloudFormation Parameters
const config = new pulumi.Config();
const sesVerifiedEmail = config.require("SesVerifiedEmail");
const oidcDomain = config.require("OidcDomain");
const oidcStage = config.require("OidcStage");
const loginGovClientId = config.require("LoginGovClientId");
const loginGovIssuer = config.get("LoginGovIssuer") || "https://idp.int.identitysandbox.gov";
const resourcePrefix = 'PrecheckCognito';

const kmsKey = new aws.kms.Key(`${resourcePrefix}KmsKey${resourceSuffix}`, {
    description: "KMS key for Cognito custom SMS sender",
    keyUsage: "ENCRYPT_DECRYPT",
    deletionWindowInDays: 7,
});

// Create an Alias for the KMS Key
const kmsKeyAlias = new aws.kms.Alias(`${resourcePrefix}KmsKeyAlias${resourceSuffix}`, {
    targetKeyId: kmsKey.id, // Reference the KMS key
    name: `alias/${resourcePrefix.toLowerCase()}-${resourceSuffix.toLocaleLowerCase()}-code`,   // Replace "my-key" with your desired alias
});

const lambdaRole = new aws.iam.Role(`${resourcePrefix}LambdaRole${resourceSuffix}`, {
    assumeRolePolicy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [
            {
                Action: "sts:AssumeRole",
                Effect: "Allow",
                Sid: "",
                Principal: {
                    Service: "lambda.amazonaws.com",
                },
            },
        ],
    }),
});

// Policies for Lambda IAM Role
const lambdaPolicy = new aws.iam.RolePolicy(`${resourcePrefix}LambdaPolicy${resourceSuffix}`, {
    role: lambdaRole.id,
    policy: pulumi.all([sesVerifiedEmail, kmsKey.arn]).apply(([verifiedEmail, arn]) =>
        JSON.stringify({
            Version: "2012-10-17",
            Statement: [
                {
                    Effect: "Allow",
                    Action: ["sns:Publish"],
                    Resource: "*", // Allow SNS for SMS sending
                },
                {
                    Effect: "Allow",
                    Action: ["ses:SendEmail"],
                    Resource: "*", // Allow SES for email fallback
                },
                {
                    Effect: "Allow",
                    Action: [
                        "logs:CreateLogGroup",
                        "logs:CreateLogStream",
                        "logs:PutLogEvents",
                    ],
                    Resource: "*", // Allow CloudWatch logging
                },
                {
                    Effect: "Allow",
                    Action: [
                        "kms:Decrypt",
                        "kms:Encrypt",
                        "kms:GenerateDataKey",
                        "kms:DescribeKey"
                    ],
                    Resource: arn,
                },
                {
                    Effect: "Allow",
                    Action: [
                        "cognito-idp:AdminUpdateUserAttributes", // Allow updating user attributes
                    ],
                    Resource: `*`,
                },
            ],
        })
    ),
});

// Attach policies to allow the Lambda function to log to CloudWatch
const lambdaPolicyAttach = new aws.iam.RolePolicyAttachment(`${resourcePrefix}LambdaPolicy${resourceSuffix}`, {
    role: lambdaRole.name,
    policyArn: aws.iam.ManagedPolicies.AWSLambdaBasicExecutionRole,
});

// Create a KMS Key Policy with AWS account root user permissions
const kmsKeyPolicy = new aws.kms.KeyPolicy(`${resourcePrefix}KmsKeyPolicy${resourceSuffix}`, {
    keyId: kmsKey.id,
    policy: pulumi.all([kmsKey.arn, aws.getCallerIdentity()]).apply(([keyArn, callerIdentity]) =>
        JSON.stringify({
            Version: "2012-10-17",
            Statement: [
                {
                    Sid: "AllowRootAccountAccess",
                    Effect: "Allow",
                    Principal: {
                        AWS: `arn:aws:iam::${callerIdentity.accountId}:root`, // Root account access
                    },
                    Action: "kms:*",
                    Resource: "*",
                },
                {
                    Effect: "Allow",
                    Principal: {
                        Service: "cognito-idp.amazonaws.com",
                    },
                    Action: [
                        "kms:Encrypt",
                        "kms:Decrypt",
                        "kms:GenerateDataKey",
                        "kms:DescribeKey"
                    ],
                    Resource: keyArn
                },
                {
                    
                    Effect: "Allow",
                    Principal: "*",
                    Action: ["sns:Publish"],
                    Resource: "*", // Allow SNS for SMS sending
                },
                {
                    Effect: "Allow",
                    Principal: "*",
                    Action: ["ses:SendEmail"],
                    Resource: "*", // Allow SES for email fallback
                },
                {
                    Effect: "Allow",
                    Principal: "*",
                    Action: [
                        "logs:CreateLogGroup",
                        "logs:CreateLogStream",
                        "logs:PutLogEvents",
                    ],
                    Resource: "*", // Allow CloudWatch logging
                },
                {
                    Effect: "Allow",
                    Principal: {
                        Service: "cognito-idp.amazonaws.com",
                    },
                    Action: [
                        "kms:Encrypt",
                        "kms:Decrypt",
                        "kms:GenerateDataKey",
                        "kms:DescribeKey",
                    ],
                    Resource: keyArn,
                },
            ]
        })
    ),
});

const snsCallerRole = new aws.iam.Role(`${resourcePrefix}SmsCallerRole${resourceSuffix}`, {
    assumeRolePolicy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [
            {
                Effect: "Allow",
                Principal: {
                    Service: "cognito-idp.amazonaws.com", // Allow Cognito to assume this role
                },
                Action: "sts:AssumeRole",
            },
        ],
    }),
});

const snsCallerRolePolicy = new aws.iam.RolePolicy("snsCallerRolePolicy", {
    role: snsCallerRole.name, // Replace with the actual name of your SNS Caller Role
    policy: kmsKey.arn.apply((arn) =>
        JSON.stringify({
            Version: "2012-10-17",
            Statement: [
                {
                    Effect: "Allow",
                    Action: [
                        "sns:Publish", // Allow publishing to SNS
                        "sns:CreateTopic",
                        "sns:Subscribe",
                    ],
                    Resource: "*", // Allow publishing to all SNS topics (or restrict this to specific topics)
                },
                {
                    Effect: "Allow",
                    Action: [
                        "kms:Encrypt",
                        "kms:Decrypt",
                        "kms:GenerateDataKey",
                        "kms:DescribeKey",
                    ],
                    Resource: arn, // Restrict access to the specific KMS Key
                },
            ],
        })
    ),
});

// Lambda Function for SMS with Email Fallback
const lambdaFunction = new aws.lambda.Function(`${resourcePrefix}SmsEmailLambda${resourceSuffix}`, {
    runtime: "nodejs18.x",
    handler: "index.handler",
    role: lambdaRole.arn,
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./message"), // Path to Lambda code directory
    }),
    environment: {
        variables: {
            SES_VERIFIED_EMAIL: sesVerifiedEmail,
            KEY_ARN: kmsKey.arn,
            KEY_ALIAS: kmsKeyAlias.name,
            REGION: aws.config.region,
        },
    },
    timeout: 10, // Timeout in seconds
});

// Create the Post Confirmation Lambda Function
const postConfirmationLambda = new aws.lambda.Function(`${resourcePrefix}PostConfirmationLambda${resourceSuffix}`, {
    runtime: "nodejs18.x",
    handler: "index.handler",
    role: lambdaRole.arn, // Reuse the Lambda IAM role with permissions for Cognito
    code: new pulumi.asset.AssetArchive({
        ".": new pulumi.asset.FileArchive("./post-confirmation"), // Path to Lambda code directory
    }),
    environment: {
        variables: {
            REGION: aws.config.region,
        },
    },
    timeout: 10,
});

// Cognito User Pool
const userPool = new aws.cognito.UserPool(`${resourcePrefix}UserPool${resourceSuffix}`, {
    name: `${resourcePrefix}UserPool${resourceSuffix}`,
    usernameAttributes: ["email"],
    autoVerifiedAttributes: ["email", 'phone_number'],  
    passwordPolicy: {
        minimumLength: 8,
        requireLowercase: true,
        requireNumbers: true,
        requireSymbols: true,
        requireUppercase: true,
    },
    accountRecoverySetting: {
        recoveryMechanisms: [
            {
                name: "verified_email",
                priority: 1,
            },
            {
                name: "verified_phone_number",
                priority: 2,
            },
        ],
    },
    mfaConfiguration: "ON", // Enforce MFA
    verificationMessageTemplate: {
        defaultEmailOption: "CONFIRM_WITH_CODE", // Use confirmation code via email
        smsMessage: "Your verification code is {####}.",
    },
    lambdaConfig: {
        postConfirmation: postConfirmationLambda.arn,
        customSmsSender: {
            lambdaArn: lambdaFunction.arn, // Attach the Lambda function to handle SMS sending
            lambdaVersion: "V1_0",        // Use version V1_0 of the Lambda integration
        },
        kmsKeyId: kmsKey.arn,
    },
    schemas: [
        {
            name: "email",
            attributeDataType: "String",
            mutable: true,
            required: true,
        },
        {
            name: "loginGovId",
            attributeDataType: "String",
            mutable: true,
            required: false,
        },
        {
            name: "snowCustomerId",
            attributeDataType: "String",
            mutable: true,
            required: false,
        },
        {
            name: "given_name", // First name
            attributeDataType: "String",
            mutable: true,
            required: false,
        },
        {
            name: "family_name", // Last name
            attributeDataType: "String",
            mutable: true,
            required: false,
        },
        {
            name: "middle_name", // Middle name
            attributeDataType: "String",
            mutable: true,
            required: false,
        },
        {
            name: "phone_number", // Phone number
            attributeDataType: "String",
            mutable: true,
            required: true,
        },
        {
            name: "suffix", // Suffix (custom attribute)
            attributeDataType: "String",
            mutable: true,
            required: false,
        }
    ],
    emailConfiguration: {
        emailSendingAccount: "DEVELOPER",
        fromEmailAddress: sesVerifiedEmail,
        sourceArn: pulumi.interpolate`arn:aws:ses:${aws.config.region}:${aws.getCallerIdentity().then(c => c.accountId)}:identity/${sesVerifiedEmail}`,
    },
    smsConfiguration: {
        snsCallerArn: snsCallerRole.arn, // Use the SNS caller role with the correct trust relationship
        externalId: "dummyExternalId",   // Optional: External ID for additional security
        snsRegion: aws.config.region,
    },
}, {
    dependsOn:[snsCallerRole, lambdaFunction, kmsKey, postConfirmationLambda]
});

const lambdaPermission = new aws.lambda.Permission(`${resourcePrefix}LambdaPermission${resourceSuffix}`, {
    action: "lambda:InvokeFunction",
    function: lambdaFunction.arn,
    principal: "cognito-idp.amazonaws.com",
    sourceArn: userPool.arn,
}, {
    dependsOn: [lambdaFunction, userPool]
});

const postConfirmationLambdaPermission = new aws.lambda.Permission(`${resourcePrefix}PostConfirmationLambdaPermission${resourceSuffix}`, {
    action: "lambda:InvokeFunction",
    function: postConfirmationLambda.arn,
    principal: "cognito-idp.amazonaws.com",
    sourceArn: userPool.arn,
}, {
    dependsOn: [postConfirmationLambda, userPool]
});

// Cognito User Pool Identity Provider for Login.gov
const userPoolIdentityProviderLoginGovOidc = new aws.cognito.IdentityProvider(`${resourcePrefix}LoginGovIdentityProvider${resourceSuffix}`, {
    providerName: "LoginGov",
    providerType: "OIDC",
    userPoolId: userPool.id,
    providerDetails: {
        client_id: loginGovClientId,
        authorize_scopes: "openid email",
        attributes_request_method: "GET",
        oidc_issuer: loginGovIssuer,
        jwks_uri: "https://idp.int.identitysandbox.gov/api/openid_connect/certs",
        token_url: pulumi.interpolate`https://${oidcDomain}/${oidcStage}/token`,
        authorize_url: pulumi.interpolate`https://${oidcDomain}/${oidcStage}/login`,
        attributes_url: pulumi.interpolate`https://${oidcDomain}/${oidcStage}/user`,
    },
    attributeMapping: {
        email: "email",
        name: "email",
        "custom:loginGovId":"sub",
        phone_number: "+12345678910",
    },
});

// Cognito User Pool Client
const userPoolClient = new aws.cognito.UserPoolClient(`${resourcePrefix}UserPoolClient${resourceSuffix}`, {
    name: `${resourcePrefix}UserPoolClient${resourceSuffix}`,
    userPoolId: userPool.id,
    generateSecret: false,
    supportedIdentityProviders: ["COGNITO", "LoginGov"],
    callbackUrls: ["http://localhost:4200/auth-callback"],
    logoutUrls: ["http://localhost:4200/logout"],
    allowedOauthFlows: ["code"],
    allowedOauthScopes: ["email", "openid"],
    allowedOauthFlowsUserPoolClient: true,
    explicitAuthFlows:['ALLOW_REFRESH_TOKEN_AUTH', 'ALLOW_USER_SRP_AUTH', 'ALLOW_CUSTOM_AUTH', 'ALLOW_USER_PASSWORD_AUTH']
}, {
    dependsOn: [userPoolIdentityProviderLoginGovOidc]
});

// Cognito Identity Pool
const identityPool = new aws.cognito.IdentityPool(`${resourcePrefix}IdentityPool${resourceSuffix}`, {
    identityPoolName: `${resourcePrefix}IdentityPool${resourceSuffix}`,
    allowUnauthenticatedIdentities: true,
    cognitoIdentityProviders: [
        {
            clientId: userPoolClient.id,
            providerName: pulumi.interpolate`cognito-idp.${aws.config.region}.amazonaws.com/${userPool.id}`,
        },
    ],
});

// Authenticated Role
const authenticatedRole = new aws.iam.Role(`${resourcePrefix}AuthenticatedRole${resourceSuffix}`, {
    name: `${resourcePrefix}AuthenticatedRole${resourceSuffix}`,
    assumeRolePolicy: identityPool.id.apply(identityPoolId => JSON.stringify({
        Version: "2012-10-17",
        Statement: [
            {
                Effect: "Allow",
                Principal: { Federated: "cognito-identity.amazonaws.com" },
                Action: "sts:AssumeRoleWithWebIdentity",
                Condition: {
                    StringEquals: {
                        "cognito-identity.amazonaws.com:aud": identityPoolId // Resolves the Output<string> to a plain string
                    },
                    "ForAnyValue:StringLike": {
                        "cognito-identity.amazonaws.com:amr": "authenticated"
                    }
                }
            }
        ]
    }))
});

// Unauthenticated Role
const unauthenticatedRole = new aws.iam.Role(`${resourcePrefix}UnauthenticatedRole${resourceSuffix}`, {
    name: `${resourcePrefix}UnauthenticatedRole${resourceSuffix}`,
    assumeRolePolicy: identityPool.id.apply(identityPoolId => JSON.stringify({
        Version: "2012-10-17",
        Statement: [
            {
                Effect: "Allow",
                Principal: { Federated: "cognito-identity.amazonaws.com" },
                Action: "sts:AssumeRoleWithWebIdentity",
                Condition: {
                    StringEquals: {
                        "cognito-identity.amazonaws.com:aud": identityPoolId // Resolves the Output<string> for the Identity Pool ID
                    },
                    "ForAnyValue:StringLike": {
                        "cognito-identity.amazonaws.com:amr": "unauthenticated" // Specifies unauthenticated users only
                    }
                }
            }
        ]
    }))
});

// Identity Pool Role Attachment
const identityPoolRoleAttachment = new aws.cognito.IdentityPoolRoleAttachment(
    `${resourcePrefix}IdentityPoolRoleAttachment${resourceSuffix}`,
    {
        identityPoolId: identityPool.id,
        roles: {
            authenticated: authenticatedRole.arn,
            unauthenticated: unauthenticatedRole.arn,
        },
    }
);

// Cognito User Pool Domain
const userPoolDomain = new aws.cognito.UserPoolDomain(`${resourcePrefix}UserPoolDomain${resourceSuffix}`, {
    domain: `precheck-${pulumi.getStack().toLowerCase()}`,
    userPoolId: userPool.id,
},{
    dependsOn: [ 
        userPool,
        userPoolClient
    ]
});

// Outputs
export const userPoolId = userPool.id;
export const userPoolClientId = userPoolClient.id;
export const identityPoolId = identityPool.id;
export const lambdaFunctionName = lambdaFunction.name;
export const lambdaFunctionArn = lambdaFunction.arn;
export { loginGovClientId, loginGovIssuer };