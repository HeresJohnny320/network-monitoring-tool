package hj320.network_tool;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;

public class database {

    private static database instance;

    private boolean runSql;
    private String url;
    private String user;
    private String password;

    private database(readjson reader) {
        try {
            runSql = reader.getBoolean("run_sql", false);

            if (!runSql) {
                url = "jdbc:sqlite:database.db";
                user = null;
                password = null;
            } else {
                user = reader.getString("sql_user", "user");
                password = reader.getString("sql_password", "password");
                String host = reader.getString("sql_host", "localhost");
                String port = reader.getString("sql_port", "3306");
                String database = reader.getString("sql_database", "defaultdb");
                url = "jdbc:mysql://" + host + ":" + port + "/" + database;
            }
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    public static database init(readjson reader) {
        if (instance == null) {
            instance = new database(reader);
        }
        return instance;
    }

    public static database getInstance() {
        if (instance == null) {
            throw new RuntimeException("database not initialized. Call database.init(reader) first.");
        }
        return instance;
    }

    public Connection getConnection() throws SQLException {
        if (runSql) {
            return DriverManager.getConnection(url, user, password);
        } else {
            return DriverManager.getConnection(url);
        }
    }
}