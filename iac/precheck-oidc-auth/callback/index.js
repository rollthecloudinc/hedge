const AWS = require("aws-sdk");
const fetch = require("node-fetch");
const jwt = require("jsonwebtoken");
const jwksClient = require("jwks-rsa");
const logger = require("./logger");

const dynamodb = new AWS.DynamoDB.DocumentClient();

exports.handler = async (event) => {
    try {

        // Extract query string parameters (e.g., code and state)
        const { queryStringParameters } = event;
        const { code, state } = queryStringParameters || {};

        logger({ state });

        // Validate the state
        if (!state) {
            console.error("Missing state in callback");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Missing state" }),
            };
        }

        // Retrieve the state and codeVerifier from DynamoDB
        const stateData = await dynamodb
            .get({
                TableName: process.env.STATE_TABLE_NAME,
                Key: { state },
            })
            .promise();

        if (!stateData.Item) {
            console.error("Invalid or expired state");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Invalid or expired state" }),
            };
        }

        const { codeVerifier, nonce } = stateData.Item;

        // Delete the state from DynamoDB after validation
        await dynamodb
            .delete({
                TableName: process.env.STATE_TABLE_NAME,
                Key: { state },
            })
            .promise();


        // Retrieve the redirect URL from the RedirectTable
        const redirectData = await dynamodb
            .get({
                TableName: process.env.REDIRECT_TABLE_NAME,
                Key: { state },
            })
            .promise();

        if (!redirectData.Item) {
            console.error("Missing or expired redirect URL");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Missing or expired redirect URi" }),
            };
        }

        const { redirectUri } = redirectData.Item;

        // Clean up the redirect URL entry from DynamoDB
        await dynamodb
            .delete({
                TableName: process.env.REDIRECT_TABLE_NAME,
                Key: { state },
            })
            .promise();

        // Exchange the authorization code for tokens
        const tokenResponse = await fetch(process.env.LOGIN_GOV_TOKEN_URL, {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },
            body: new URLSearchParams({
                client_id: process.env.LOGIN_GOV_CLIENT_ID,
                grant_type: "authorization_code",
                code,
                redirect_uri: process.env.LOGIN_GOV_REDIRECT_URI,
                code_verifier: codeVerifier,
            }),
        });

        if (!tokenResponse.ok) {
            const errorText = await tokenResponse.text();
            console.error("Error exchanging token:", errorText);
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Failed to exchange token", details: errorText }),
            };
        }

        const tokens = await tokenResponse.json();

        // Validate the ID token using `jwks-rsa`
        if (tokens.id_token) {
            const client = jwksClient({
                jwksUri: `${process.env.LOGIN_GOV_ISSUER}/api/openid_connect/certs`,
            });

            const getKey = (header, callback, options) => {
                client.getSigningKey(header.kid, (err, key) => {
                    if (err) {
                        callback(err);
                    } else {
                        const signingKey = key.getPublicKey();
                        callback(null, signingKey);
                    }
                });
            };

            // Verify the ID token
            try {
                const decodedToken = await new Promise((resolve, reject) => {
                    jwt.verify(
                        tokens.id_token,
                        getKey,
                        {
                            algorithms: ["RS256"],
                            audience: process.env.LOGIN_GOV_CLIENT_ID,
                            // issuer: process.env.LOGIN_GOV_ISSUER,
                        },
                        (err, decoded) => {
                            if (err) {
                                return reject(err);
                            }
                            resolve(decoded);
                        }
                    );
                });

                // Validate the nonce in the ID token
                if (decodedToken.nonce !== nonce) {
                    return {
                        statusCode: 401,
                        body: JSON.stringify({ error: "Invalid nonce in ID token" }),
                    };
                }

                console.log("ID Token successfully validated:", decodedToken);
            } catch (error) {
                console.error("Error validating ID Token:", error.message);
                return {
                    statusCode: 401,
                    body: JSON.stringify({
                        error: "Invalid ID token",
                        details: error.message,
                    }),
                };
            }
        }

        // Generate a new authorization code for Cognito
        const cognitoAuthCode = generateRandomString();

        // Store the new authorization code with the associated user data (e.g., ID token claims)
        await dynamodb
            .put({
                TableName: process.env.AUTH_CODE_TABLE_NAME,
                Item: {
                    auth_code: cognitoAuthCode,
                    id_token: tokens.id_token,
                    access_token: tokens.access_token,
                    refresh_token: tokens.refresh_token,
                    expiration: Math.floor(Date.now() / 1000) + 300, // 5-minute TTL
                },
            })
            .promise();

        // Redirect back to Cognito with form encodded authorization code and state
        const idpParams = new URLSearchParams();
        idpParams.append('code', cognitoAuthCode);
        idpParams.append('state', state);
        const cognitoRedirectUrl = `${redirectUri}?${idpParams.toString()}`;

        return {
            statusCode: 302,
            headers: {
                Location: cognitoRedirectUrl
            },
        };
    } catch (error) {
        console.error("Error in token exchange:", error);
        return {
            statusCode: 500,
            body: JSON.stringify({ error: "Internal server error", details: error.message }),
        };
    }
};

// Helper function to generate a secure random string
function generateRandomString(length = 32) {
    return [...Array(length)]
        .map(() => Math.floor(Math.random() * 36).toString(36))
        .join("");
}