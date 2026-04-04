package hj320.network_tool;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.text.SimpleDateFormat;
import java.util.*;


@RestController
@RequestMapping("/ping")
public class ping {

    static class PingResult {
        boolean success;
        long timeMs;
        String raw;
        String hosts;

        PingResult(boolean success, long timeMs, String raw, String hosts) {
            this.success = success;
            this.timeMs = timeMs;
            this.raw = raw;
            this.hosts = hosts;
        }
    }
    public static void pingHosts(List<String> hosts) {
        if (hosts == null || hosts.isEmpty()) return;

        for (String host : hosts) {
            try {
                PingResult result = pingHost(host);

                String date = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss").format(new Date());
                Map<String, Object> result2 = new LinkedHashMap<>();
                result2.put("host", result.hosts);
                result2.put("pass", result.success);
                result2.put("timeMs", result.timeMs);
                result2.put("date", date);

                System.out.println(result2);
                try (Connection conn = database.getInstance().getConnection();
                     PreparedStatement stmt = conn.prepareStatement(
                             "INSERT INTO ping_results (host, pass, time_ms, timestamp) VALUES (?, ?, ?, ?)")) {

                    stmt.setString(1, result.hosts);
                    stmt.setInt(2, result.success ? 1 : 0);
                    stmt.setLong(3, result.timeMs);
                    stmt.setString(4, date);

                    stmt.executeUpdate();
                } catch (Exception e) {
                    e.printStackTrace();
                }

            } catch (Exception e) {
                e.printStackTrace();
            }
        }
    }
    public static PingResult pingHost(String host) {
        String os = System.getProperty("os.name").toLowerCase();
        String pingCmd = "";

        if (os.contains("win")) {
            pingCmd = "ping -n 1 " + host;
        } else {
            pingCmd = "ping -c 1 " + host;
        }

        boolean success = false;
        long timeMs = -1;
        String input2 = null;
        try {
            Process process = Runtime.getRuntime().exec(pingCmd);
            BufferedReader input = new BufferedReader(new InputStreamReader(process.getInputStream()));
            String line;
            while ((line = input.readLine()) != null) {
                line = line.toLowerCase();
                input2 = line;
                if (line.contains("time=")) {
                    success = true;
                    int index = line.indexOf("time=");
                    int endIndex = line.indexOf("ms", index);
                    if (endIndex == -1) {
                        endIndex = line.indexOf(" ", index);
                    }
                    String timeStr = line.substring(index + 5, endIndex).replaceAll("[^0-9.]", "");
                    timeMs = Math.round(Double.parseDouble(timeStr));
                }
            }

        } catch (Exception e) {
            System.out.println("Ping error: " + e.getMessage());
        }

        return new PingResult(success, timeMs, input2, host);
    }


    public static class pingdbresult {
        public String host;
        public String success;
        public String timems;
        public String timestamp;

        public pingdbresult(String host, String success, String timems, String timestamp) {
            this.host = host;
            this.success = success;
            this.timems = timems;
            this.timestamp = timestamp;
        }
    }
        @GetMapping
        public List<pingdbresult> getAll() {
            try (Connection conn = database.getInstance().getConnection()) {
                List<pingdbresult> results = new ArrayList<>();
                String selectSql = "SELECT * FROM ping_results";
                try (PreparedStatement stmt = conn.prepareStatement(selectSql);
                     ResultSet rs = stmt.executeQuery()) {

                    while (rs.next()) {
                        results.add(new pingdbresult(
                                rs.getString("host"),
                                rs.getInt("pass") == 1 ? "true" : "false",
                                rs.getString("time_ms"),
                                rs.getString("timestamp")
                        ));
                    }
                }
                return results;
            } catch (SQLException e) {
                throw new RuntimeException(e);
            }
        }

}
