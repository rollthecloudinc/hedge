const AWS = require("aws-sdk");
const querystring = require("querystring"); // Used to parse form-encoded data
const logger = require("./logger");

const dynamodb = new AWS.DynamoDB.DocumentClient();

exports.handler = async (event) => {
    try {
        // Parse the form-encoded body
        const body = querystring.parse(event.body);
        const { code /*, state , redirect_uri, client_id*/ } = body;

        logger({});

        // Validate the input parameters
        if (!code) {
            console.error("Missing authorization code");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Missing authorization code" }),
            };
        }

        // Retrieve the token data from DynamoDB using the authorization code
        const tokenData = await dynamodb
            .get({
                TableName: process.env.AUTH_CODE_TABLE_NAME,
                Key: { auth_code: code },
            })
            .promise();

        // Check if token data exists
        if (!tokenData.Item) {
            console.error("Invalid or expired authorization code");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Invalid or expired authorization code" }),
            };
        }

        // No state provided in request for cognito but maybe others so leave it here
        /*if (tokenData.Item.state !== state) {
            console.error("State mismatch");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "State mismatch" }),
            };
        }*/

        // Optional: Validate redirect_uri
        /*if (redirect_uri && tokenData.Item.redirect_uri !== redirect_uri) {
            console.error("Redirect URI mismatch");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Redirect URI mismatch" }),
            };
        }*/

        // Optional: Validate client_id
        /*if (client_id && process.env.COGNITO_APP_CLIENT_ID !== client_id) {
            console.error("Client ID mismatch");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Client ID mismatch" }),
            };
        }*/

        // Extract the tokens from the retrieved data
        const { id_token, access_token, refresh_token, expiration } = tokenData.Item;

        // Optional: Check if the tokens are expired
        const currentTimestamp = Math.floor(Date.now() / 1000);
        if (expiration < currentTimestamp) {
            console.error("Authorization code has expired");
            return {
                statusCode: 400,
                body: JSON.stringify({ error: "Authorization code has expired" }),
            };
        }

        // Return the tokens to the client
        return {
            statusCode: 200,
            body: JSON.stringify({
                id_token,
                access_token,
                refresh_token,
                expires_in: expiration - currentTimestamp, // Remaining time until expiration
                token_type: 'Bearer'
            }),
        };
    } catch (error) {
        console.error("Error retrieving token:", error);
        return {
            statusCode: 500,
            body: JSON.stringify({ error: "Internal server error", details: error.message }),
        };
    }
};