package hj320.network_tool;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.io.IOException;
import java.sql.SQLException;
@SpringBootApplication
@RestController
public class Main {
    public static void main(String[] args) {
        try {
            if(check_depend.dependisinstalled() == false)return;
            if(check_depend.dependisinstalled() == true){System.out.println("Everything needed Installed :)");}
            readjson reader = new readjson();
            database.init(reader);
            createdatabase();
            SpringApplication.run(Main.class, args);
            pages.mainpage();
            String runevery = reader.getString("run_every", "1h");
            System.out.println("cron started running every "+runevery);
            timerutil.scheduleRepeating(runevery, () -> {
                runnetworklog(reader);
            });
        } catch (IOException e) {
            throw new RuntimeException(e);
        }


    }

    private static void runnetworklog(readjson reader){
        try {
            System.out.println("Running scheduled ping...");
            pings(reader);
            System.out.println("Running scheduled traceroute...");
            traceroute(reader);
            System.out.println("Running scheduled startspeed...");
            startspeed(reader);
            System.out.println("DONE");
        } catch (Exception e) {
            e.printStackTrace();
        }

    }
    private static void pings(readjson reader){
        try {
            ping.pingHosts(reader.getStringList("ping_host"));
        } catch (Exception e) {
            e.printStackTrace();
        }

    }

    private static void traceroute(readjson reader){
        traceroute traceroute = new traceroute();
        try {
            traceroute.runTraceroute(reader.getStringList("traceroute_host"));
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
    private static void startspeed(readjson reader){
        speedtest speedtest = new speedtest();
        try {
            String serverId = reader.getString("speedtest_server_id", "");
            speedtest.speedtest(serverId);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
    private static void createdatabase(){
        try {
            var data = database.getInstance().getConnection().createStatement();
                String pingTable = """
                CREATE TABLE IF NOT EXISTS ping_results (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    host TEXT NOT NULL,
                    pass BOOLEAN NOT NULL,
                    time_ms INTEGER NOT NULL,
                    timestamp DATETIME NOT NULL
                );
            """;

                String tracerouteTable = """
                CREATE TABLE IF NOT EXISTS traceroute_results (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    host TEXT NOT NULL,
                    timestamp DATETIME NOT NULL
                );
            """;

                String tracerouteHopsTable = """
                CREATE TABLE IF NOT EXISTS traceroute_hops (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    traceroute_id INTEGER NOT NULL,
                    hop_number INTEGER NOT NULL,
                    ip TEXT,
                    time1_ms REAL,
                    time2_ms REAL,
                    time3_ms REAL,
                    FOREIGN KEY(traceroute_id) REFERENCES traceroute_results(id)
                );
            """;

                String speedtestTable = """
                CREATE TABLE IF NOT EXISTS speedtest_results (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    download REAL NOT NULL,
                    upload REAL NOT NULL,
                    ping REAL NOT NULL,
                    server_id INTEGER,
                    server_host TEXT,
                    server_location TEXT,
                    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
                );
            """;

            data.executeUpdate(pingTable);
            data.executeUpdate(tracerouteTable);
            data.executeUpdate(tracerouteHopsTable);
            data.executeUpdate(speedtestTable);
        } catch (SQLException e) {
            throw new RuntimeException(e);
        }
    }
}



