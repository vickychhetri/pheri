SELECT table_name AS name, 'TABLE' AS type 
						FROM information_schema.tables 
						WHERE table_schema = 'stohrm_cocacola' AND table_type = 'BASE TABLE'
						UNION ALL
						SELECT table_name AS name, 'VIEW' AS type 
						FROM information_schema.tables 
						WHERE table_schema = 'stohrm_cocacola' AND table_type = 'VIEW'
						UNION ALL
						SELECT routine_name AS name, 'PROCEDURE' AS type 
						FROM information_schema.routines 
						WHERE routine_schema = 'stohrm_cocacola' AND routine_type = 'PROCEDURE'
						UNION ALL
						SELECT routine_name AS name, 'FUNCTION' AS type 
						FROM information_schema.routines 
						WHERE routine_schema = 'stohrm_cocacola' AND routine_type = 'FUNCTION';