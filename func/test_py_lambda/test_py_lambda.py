def lambda_handler(event, context):
    message = 'Hello, World!'
    
    return {
        'statusCode': 200,
        'body': message
    }