go test -v -db_drv=odbc_with_oracle -db_url=DSN=tpt_oracle;UID=system;PWD=123456
go test -v -db_drv=oci8 -db_url=system/123456@TPT -test.run=TestEnqueue