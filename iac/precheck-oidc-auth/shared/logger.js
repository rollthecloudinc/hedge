// Helper function to format log messages with context variables
const format = (level, contextVars, message, additionalData) => {
    const timestamp = new Date().toISOString();
    const contextString = Object.entries(contextVars)
        .map(([key, value]) => `${key}=${value}`)
        .join(" ");
    const additionalString = additionalData ? JSON.stringify(additionalData) : "";
    return `${timestamp} [${level}] ${contextString} ${message} ${additionalString}`;
};

// Functional logger setup to override console methods
const logger = (contextVars) => {
    const log = console.log;
    const error = console.error;
    const warn = console.warn;
    const debug = console.debug;

    // Wrap each console method with custom formatting
    console.log = (message, additionalData) =>
        log(format("INFO", contextVars, message, additionalData));

    console.error = (message, additionalData) =>
        error(format("ERROR", contextVars, message, additionalData));

    console.warn = (message, additionalData) =>
        warn(format("WARN", contextVars, message, additionalData));

    console.debug = (message, additionalData) =>
        debug(format("DEBUG", contextVars, message, additionalData));
};

// Export the setup function
module.exports = logger;