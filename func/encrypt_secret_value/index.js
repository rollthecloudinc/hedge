const sodium = require("libsodium-wrappers");

module.exports.handler = async (event) => {
    try {
        await sodium.ready;

        const { publicKey: publicKeyBase64, secretValue } = event;

        // Validate the inputs
        if (!publicKeyBase64 || typeof publicKeyBase64 !== "string") {
            throw new Error(`Invalid or missing publicKey: ${publicKeyBase64}`);
        }
        if (!secretValue || typeof secretValue !== "string") {
            throw new Error(`Invalid or missing secretValue: ${secretValue}`);
        }

        // Decode public key
        const publicKey = Buffer.from(publicKeyBase64, "base64");
        if (publicKey.length !== sodium.crypto_box_PUBLICKEYBYTES) {
            throw new Error(
                `Invalid publicKey length: Expected ${sodium.crypto_box_PUBLICKEYBYTES}, got ${publicKey.length}`
            );
        }

        // Encrypt the message
        const message = Buffer.from(secretValue, "utf8");
        const sealedBox = sodium.crypto_box_seal(message, publicKey);

        // Convert the sealed box to Base64
        const encryptedValue = Buffer.from(sealedBox).toString("base64");

        // Ensure the encryptedValue is not empty
        if (!encryptedValue) {
            throw new Error("Failed to encrypt secretValue; Empty encryptedValue.");
        }

        console.log("Encrypted Value:", encryptedValue);
        return {
            statusCode: 200,
            body: JSON.stringify({ encryptedValue }),
        };
    } catch (error) {
        console.error("Lambda error:", error.message);
        return {
            statusCode: 500,
            body: JSON.stringify({ message: `Error processing request: ${error.message}` }),
        };
    }
};