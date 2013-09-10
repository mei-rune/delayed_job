go test -v -test_db_drv=odbc_with_oracle -test_db_url=DSN=tpt_oracle;UID=system;PWD=123456 -test.run=DbHandler
go test -v -test_db_drv=oci8 -test_db_url=system/123456@TPT -test.run=DbHandler