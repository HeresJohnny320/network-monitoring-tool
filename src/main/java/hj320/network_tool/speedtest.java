package hj320.network_tool;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.web.bind.annotation.*;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;

import static java.sql.DriverManager.getConnection;

@RestController
@RequestMapping("/speedtest")
public class speedtest {

    public static void speedtest(String server) {
        System.out.println("Running speed test...");
        try {
            ProcessBuilder pb;
            String os = System.getProperty("os.name").toLowerCase();

            if (os.contains("win")) {
                if (server != null && !server.isEmpty()) {
                    pb = new ProcessBuilder("speedtest", "--server-id", server, "--format=json");
                } else {
                    pb = new ProcessBuilder("speedtest", "--format=json");
                }
            } else {
                if (server != null && !server.isEmpty()) {
                    pb = new ProcessBuilder("speedtest", "--server", server, "--json");
                } else {
                    pb = new ProcessBuilder("speedtest", "--json");
                }
            }

            pb.redirectErrorStream(true);
            Process process = pb.start();

            BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
            StringBuilder output = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) {
                output.append(line);
            }

            int exitCode = process.waitFor();
            if (exitCode != 0) {
                System.err.println("Speedtest CLI exited with code " + exitCode);
                return;
            }

            String json = output.toString().toLowerCase();
            ObjectMapper mapper = new ObjectMapper();
            try {
                JsonNode jsonNode = mapper.readTree(json);

                String download = "";
                String upload = "";
                String ping = "";
                String serverloc = "";
                if (os.contains("win")) {
                    download = String.valueOf(jsonNode.path("download").path("bandwidth").asLong());
                    upload = String.valueOf(jsonNode.path("upload").path("bandwidth").asLong());
                    ping = String.valueOf(jsonNode.path("ping").path("latency").asDouble());
                    serverloc = String.valueOf(jsonNode.path("server").path("location").asText(""));
                } else {
                    download = jsonNode.path("download").asText("");
                    upload = jsonNode.path("upload").asText("");
                    ping = jsonNode.path("ping").asText("");
                    serverloc = jsonNode.path("server").path("name").asText("");
                }

                JsonNode serverinfo = jsonNode.path("server");
                String serverhost = serverinfo.path("host").asText("");
                String serverid = serverinfo.path("id").asText("");

                System.out.println("download: " + download + ", upload: " + upload + ", ping: " + ping + ", serverid: " + serverid + ", serverhost: " + serverhost + ", serverloc: " + serverloc);
                saveToDatabase(download, upload, ping, serverid, serverhost, serverloc);

            } catch (Exception e) {
                e.printStackTrace();
            }
        } catch (IOException | InterruptedException e) {
            System.err.println("Error running speedtest CLI: " + e.getMessage());
            e.printStackTrace();
        }
    }

    private static void saveToDatabase(String download,String upload,String ping,String serverid,String serverhost,String serverloc) {
            try (Connection conn = database.getInstance().getConnection()) {
                String sql = "INSERT INTO speedtest_results (download, upload, ping, server_id, server_host, server_location) VALUES (?, ?, ?, ?, ?, ?)";
                try (PreparedStatement stmt = conn.prepareStatement(sql)) {
                    stmt.setString(1, download);
                    stmt.setString(2, upload);
                    stmt.setString(3, ping);
                    stmt.setString(4, serverid);
                    stmt.setString(5, serverhost);
                    stmt.setString(6, serverloc);
                    stmt.executeUpdate();
                }
        } catch (SQLException e) {
            throw new RuntimeException(e);
        }
//        System.out.println(speedTestJson);
    }

public static class SpeedtestResult {
    public String download;
    public String upload;
    public String ping;
    public String serverId;
    public String serverHost;
    public String serverLocation;
    public String timestamp;
    public SpeedtestResult(String download, String upload, String ping, String serverId, String serverHost, String serverLocation, String timestamp) {
        this.download = download;
        this.upload = upload;
        this.ping = ping;
        this.serverId = serverId;
        this.serverHost = serverHost;
        this.serverLocation = serverLocation;
        this.timestamp = timestamp;
    }
}

    @GetMapping
    public List<SpeedtestResult> getAll() {
        try (Connection conn = database.getInstance().getConnection()) {
            List<SpeedtestResult> results = new ArrayList<>();
            String selectSql = "SELECT * FROM speedtest_results";
            try (PreparedStatement stmt = conn.prepareStatement(selectSql);
                 ResultSet rs = stmt.executeQuery()) {

                while (rs.next()) {
                    results.add(new SpeedtestResult(
                            rs.getString("download"),
                            rs.getString("upload"),
                            rs.getString("ping"),
                            rs.getString("server_id"),
                            rs.getString("server_host"),
                            rs.getString("server_location"),
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

