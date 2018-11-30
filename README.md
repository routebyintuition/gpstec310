# gpstec310

This is the Unicorn Rentals reservation application. This application looks up unicorn rental 
information based upon a confirmation code. To deploy this application follow the steps below:

0) Create an S3 bucket in the region where you are running the the CloudFormation template.
    - Within that bucket, enable public web hosting to run the frontend website.
1) Copy the following zip file into that bucket:
    https://s3-us-west-2.amazonaws.com/reservations-serverless.edu.unicorns.direct/lambda/lookup.zip
2) Record the bucket name and location of this new file for use in the CloudFormation template.
3) Deploy the CloudFormation YAML template in this repository.
    Replace the LambdaZipFile parameter with your new S3 file location for lookup.zip
    Set other necessary parameters like database password and username.
4) Wait for the CloudFormation template to finalize with a status of, CREATE_COMPLETE
5) You will now have a 

Resources:

    You will have parameters setup in the AWS SSM parameter store:
    - /prod/unicorndb/mysqldb - this should be a string set to the MySQL-compatible database name
    - /prod/unicorndb/mysqlhost - this should be a string set to the MySQL-compatible hostname
    - /prod/unicorndb/mysqlpass - this is a string set to the password associated with access to the MySQL server
    - /prod/unicorndb/mysqlport - this is a string set to the TCP port number of the MySQL server
    - /prod/unicorndb/mysqluser - this is a string set to the username with access to the MySQL server
    
    