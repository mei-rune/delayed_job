go test -v -db_drv=odbc_with_mssql -db_url=DSN=odbc_with_mssql;UID=tpt;PWD=123456 %*



go test -v -db_drv=dm -db_url=dm://%dm_username%:%dm_password%@%dm_host%?noConvertToHex=true %*
