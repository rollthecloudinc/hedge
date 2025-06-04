const fetch = require("node-fetch");
const logger = require("./logger");

exports.handler = async (event) => {
    logger({});

    console.log("Received event:", JSON.stringify(event, null, 2));

    // Extract the Authorization header
    const authorizationHeader = event.headers?.Authorization || event.headers?.authorization;
    if (!authorizationHeader || !authorizationHeader.startsWith("Bearer ")) {
        return {
            statusCode: 401,
            body: JSON.stringify({ error: "Unauthorized: Missing or invalid Authorization header" }),
        };
    }

    const accessToken = authorizationHeader.split(" ")[1];

    try {
        // Fetch user information from Login.gov `/userinfo` endpoint
        const userInfoResponse = await fetch(`${process.env.LOGIN_GOV_ISSUER}/api/openid_connect/userinfo`, {
            method: "GET",
            headers: {
                Authorization: `Bearer ${accessToken}`,
            },
        });

        if (!userInfoResponse.ok) {
            console.error("Failed to fetch user info from Login.gov:", userInfoResponse.statusText);
            return {
                statusCode: userInfoResponse.status,
                body: await userInfoResponse.text(),
            };
        }

        const userInfo = await userInfoResponse.json();

        // Return the user information
        return {
            statusCode: 200,
            body: JSON.stringify(userInfo),
        };
    } catch (error) {
        console.error("Error fetching user info from Login.gov:", error);
        return {
            statusCode: 500,
            body: JSON.stringify({ error: "Failed to fetch user info", details: error.message }),
        };
    }
};