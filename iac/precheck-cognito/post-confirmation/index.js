const AWS = require("aws-sdk");

exports.handler = async (event) => {
    console.log("Event:", JSON.stringify(event, null, 2));
    
    const userPoolId = event.userPoolId;
    const userName = event.userName;
    const email = event.request.userAttributes.email;
    const phoneNumberVerified = event.request.userAttributes.phone_number_verified;

    // Only proceed if phone number is verified
    if (phoneNumberVerified === "true" && email) {
        const cognito = new AWS.CognitoIdentityServiceProvider();

        try {
            // Update the email_verified attribute to true
            await cognito.adminUpdateUserAttributes({
                UserPoolId: userPoolId,
                Username: userName,
                UserAttributes: [
                    {
                        Name: "email_verified",
                        Value: "true",
                    },
                ],
            }).promise();

            console.log(`Email verified for user: ${userName}`);
        } catch (error) {
            console.error("Error updating email_verified:", error);
            throw error;
        }
    }

    return event;
};