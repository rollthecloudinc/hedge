const AWS = require('aws-sdk');
const b64 = require('base64-js');
const encryptionSdk = require('@aws-crypto/client-node');

// Configure the encryption SDK client with the KMS key from the environment variables
const { encrypt, decrypt } = encryptionSdk.buildClient(encryptionSdk.CommitmentPolicy.REQUIRE_ENCRYPT_ALLOW_DECRYPT);
const generatorKeyId = process.env.KEY_ALIAS; // KMS Key Alias (e.g., alias/your-key)
const keyIds = [process.env.KEY_ARN]; // KMS Key ARN (e.g., arn:aws:kms:region:account-id:key/key-id)
const keyring = new encryptionSdk.KmsKeyringNode({ generatorKeyId, keyIds });

const sns = new AWS.SNS({ region: process.env.REGION }); // Default SNS region
const ses = new AWS.SES({ region: process.env.REGION }); // Default SES region

exports.handler = async (event) => {
    console.log('Received event:', JSON.stringify(event, null, 2));

    const phoneNumber = event.request.userAttributes.phone_number; // E.164 format
    const encryptedCode = event.request.code; // Encrypted verification code
    const fallbackEmail = event.request.userAttributes.email; // Fallback email, if needed

    // Ensure required attributes are present
    if (!phoneNumber || !encryptedCode) {
        console.error('Missing required parameters: phone number or encrypted code');
        throw new Error('Missing required parameters');
    }

    let plainTextCode;
    try {
        // Decrypt the secret code using the encryption SDK
        console.log('Decrypting verification code...');
        const { plaintext, messageHeader } = await decrypt(keyring, b64.toByteArray(encryptedCode));
        plainTextCode = plaintext.toString('utf-8'); // Convert plaintext buffer to a string
        console.log('Decrypted verification code:', plainTextCode);
    } catch (decryptError) {
        console.error('Failed to decrypt verification code:', decryptError);
        throw new Error('Failed to decrypt verification code');
    }

    try {
        // Step 1: Attempt to send SMS via SNS
        console.log('Attempting to send SMS via SNS...');
        const smsResult = await sns
            .publish({
                Message: `Your verification code is: ${plainTextCode}`, // The decrypted code
                PhoneNumber: phoneNumber,
            })
            .promise();

        console.log('SMS sent successfully:', smsResult);
        return event; // Return the unmodified event object to indicate success
    } catch (snsError) {
        console.error('Failed to send SMS via SNS:', snsError);

        // Step 2: Fallback to Email via SES
        if (fallbackEmail) {
            console.log('Falling back to email...');
            try {
                const emailResult = await ses
                    .sendEmail({
                        Source: process.env.SES_VERIFIED_EMAIL, // SES verified sender email
                        Destination: {
                            ToAddresses: [fallbackEmail],
                        },
                        Message: {
                            Subject: {
                                Data: 'Your Verification Code',
                            },
                            Body: {
                                Text: {
                                    Data: `We couldn't send an SMS to your phone number. Your verification code is: ${plainTextCode}`,
                                },
                            },
                        },
                    })
                    .promise();
                console.log('Fallback email sent successfully:', emailResult);
                return event; // Return the unmodified event object to indicate success
            } catch (emailError) {
                console.error('Failed to send fallback email:', emailError);
                throw new Error('Failed to send SMS and fallback email');
            }
        }

        throw new Error('Failed to send SMS and no fallback email is available');
    }
};