const crypto = require("crypto");
const AWS = require("aws-sdk");
const logger = require("./logger");

const loginGovBaseUrl = process.env.LOGIN_GOV_BASE_URL;
const loginGovClientId = process.env.LOGIN_GOV_CLIENT_ID;
const loginGovRedirectUri = process.env.LOGIN_GOV_REDIRECT_URI;
const stateTableName = process.env.STATE_TABLE_NAME;
const redirectTableName = process.env.REDIRECT_TABLE_NAME; // New table for storing redirect URLs

/**
 * Generates a random string for the PKCE code verifier.
 */
function generateCodeVerifier() {
    return crypto.randomBytes(32).toString("hex");
}

function generateNonce() {
    return crypto.randomBytes(16).toString("hex"); // Generate a random 16-byte nonce
}

/**
 * Generates a code challenge from the code verifier using SHA256.
 */
function generateCodeChallenge(codeVerifier) {
    return crypto
        .createHash("sha256")
        .update(codeVerifier)
        .digest("base64")
        .replace(/\+/g, "-")
        .replace(/\//g, "_")
        .replace(/=/g, "");
}

exports.handler = async (event) => {
    try {

        // Parse the query string to extract the original redirect URL
        const { queryStringParameters } = event;
        const originalRedirectUri = queryStringParameters?.redirect_uri;
        const state = queryStringParameters?.state;

        logger({ state });

        if (!state || state == '') {
            console.error("Missing state in query string");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Missing state in query string" }),
            };
        }

        if (!originalRedirectUri || originalRedirectUri == '') {
            console.error("Missing redirect_uri in query string");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Missing redirect_uri in query string" }),
            };
        }


        // Generate a nonce
        const nonce = generateNonce();

        // const state = crypto.randomBytes(16).toString("hex"); // Generate a random state
        const codeVerifier = generateCodeVerifier();
        const codeChallenge = generateCodeChallenge(codeVerifier);

        // Store the state and code verifier in DynamoDB
        const dynamodb = new AWS.DynamoDB.DocumentClient();

        await dynamodb
            .put({
                TableName: stateTableName,
                Item: {
                    state,
                    codeVerifier,
                    nonce,
                    ttl: Math.floor(Date.now() / 1000) + 300, // 5-minute TTL
                },
            })
            .promise();


        // Store the original redirect URL in the redirect table
        await dynamodb
            .put({
                TableName: redirectTableName,
                Item: {
                    state,
                    redirectUri: originalRedirectUri,
                    ttl: Math.floor(Date.now() / 1000) + 300, // 5-minute TTL
                },
            })
            .promise();

        // Build the Login.gov authorization URL
        const redirectUrl = `${loginGovBaseUrl}?client_id=${loginGovClientId}&response_type=code&redirect_uri=${encodeURIComponent(
            loginGovRedirectUri
        )}&scope=openid+email+verified_at+x509_presented+x509_subject&state=${state}&code_challenge=${codeChallenge}&code_challenge_method=S256&acr_values=${encodeURIComponent('urn:acr.login.gov:auth-only urn:gov:gsa:ac:classes:sp:PasswordProtectedTransport:duo')}&nonce=${nonce}`;

        // Return a redirect response
        return {
            statusCode: 302,
            headers: {
                Location: redirectUrl,
            },
        };
    } catch (error) {
        console.error("Error in Login Lambda:", error);
        return {
            statusCode: 500,
            body: JSON.stringify({ error: "Internal Server Error" }),
        };
    }
};