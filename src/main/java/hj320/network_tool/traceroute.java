package hj320.network_tool;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.sql.*;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;
import java.util.Scanner;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.io.*;
import java.text.SimpleDateFormat;
import java.util.*;
@RestController
@RequestMapping("/traceroute")
public class traceroute {

    public void runTraceroute(List<String> hosts) {
        if (hosts == null || hosts.isEmpty()) return;
        for (String host : hosts) runTraceroute(host);
    }

    public static Map<String, Object> runTraceroute(String host) {
        Map<String, Object> result = new LinkedHashMap<>();
        List<Map<String, Object>> hops = new ArrayList<>();
        result.put("host", host);
        result.put("timestamp", new SimpleDateFormat("yyyy-MM-dd HH:mm:ss").format(new Date()));

        try {
            String os = System.getProperty("os.name").toLowerCase();
            String command;

            if (os.contains("win")) {
                command = "tracert -d " + host;
            } else if (os.contains("mac") || os.contains("nix") || os.contains("nux")) {
                command = "traceroute -n " + host;
            } else {
                throw new UnsupportedOperationException("Unsupported OS: " + os);
            }

            Process process = Runtime.getRuntime().exec(command);

            try (Scanner scanner = new Scanner(process.getInputStream())) {
                boolean headerSkipped = false;
                int hopNumber = 0;
                while (scanner.hasNextLine()) {
                    String line = scanner.nextLine().trim();
                    if (line.isEmpty()) continue;

                    if (!headerSkipped) {
                        if (os.contains("win")) {
                            if (!line.matches("^\\d+\\s.*")) continue;
                        } else {
                            headerSkipped = true;
                            continue;
                        }
                        headerSkipped = true;
                    }

                    Map<String, Object> hopInfo = parseHop(line, os, ++hopNumber);
                    if (hopInfo != null) {
                        hops.add(hopInfo);
                    } else {
                        hopNumber--;
                    }
                }
            }

            process.waitFor();

        } catch (Exception e) {
            System.err.println("Error tracing host " + host + ": " + e.getMessage());
        }

        result.put("hops", hops);
        try {
            var conn = database.getInstance().getConnection();
            String sqlResult = "INSERT INTO traceroute_results (host, timestamp) VALUES (?, ?)";
            PreparedStatement stmtResult = conn.prepareStatement(sqlResult, Statement.RETURN_GENERATED_KEYS);
            stmtResult.setString(1, host);
            stmtResult.setString(2, new SimpleDateFormat("yyyy-MM-dd HH:mm:ss").format(new Date()));
            stmtResult.executeUpdate();
            ResultSet rs = stmtResult.getGeneratedKeys();
            int tracerouteId = -1;
            if (rs.next()) tracerouteId = rs.getInt(1);
            stmtResult.close();
            String sqlHop = "INSERT INTO traceroute_hops (traceroute_id, hop_number, ip, time1_ms, time2_ms, time3_ms) VALUES (?, ?, ?, ?, ?, ?)";
            PreparedStatement stmtHop = conn.prepareStatement(sqlHop);
            List<?> hopsRaw = (List<?>) result.get("hops");
            if (hopsRaw != null) {
                for (Object hopObj : hopsRaw) {
                    if (hopObj instanceof Map<?, ?> hopMapRaw) {
                        Map<?, ?> hopMap = hopMapRaw;

                        stmtHop.setInt(1, tracerouteId);
                        stmtHop.setInt(2, (int) hopMap.get("hop"));
                        stmtHop.setString(3, (String) hopMap.get("ip"));

                        List<?> times = (List<?>) hopMap.get("times");
                        for (int i = 0; i < 3; i++) {
                            if (times != null && i < times.size() && !"*".equals(times.get(i))) {
                                String t = times.get(i).toString().replace("ms", "").trim();
                                stmtHop.setDouble(4 + i, Double.parseDouble(t));
                            } else {
                                stmtHop.setNull(4 + i, java.sql.Types.REAL);
                            }
                        }

                        stmtHop.addBatch();
                    }
                }
            }

            stmtHop.executeBatch();
            stmtHop.close();

        } catch (SQLException e) {
            throw new RuntimeException(e);
        }
        System.out.println(result);
        return result;
    }

    private static Map<String, Object> parseHop(String line, String os, int hopNumber) {
        try {
            Map<String, Object> hop = new LinkedHashMap<>();
            hop.put("hop", hopNumber);

            if (os.contains("win")) {
                Pattern pattern = Pattern.compile("^\\s*(\\d+)\\s+(\\*|\\d+ ms|\\*\\s*)\\s+(\\*|\\d+ ms|\\*\\s*)\\s+(\\*|\\d+ ms|\\*\\s*)\\s+([\\d\\.]+|\\*)$");
                Matcher matcher = pattern.matcher(line);
                if (matcher.find()) {
                    String ip = matcher.group(5).trim();
                    List<String> times = new ArrayList<>();
                    String[] timeArray = {matcher.group(2), matcher.group(3), matcher.group(4)};

                    for (String time : timeArray) {
                        times.add(time.trim());
                    }

                    hop.put("ip", ip);
                    hop.put("times", times);
                    return hop;
                } else {
                    return null;
                }
            } else {

                Pattern pattern = Pattern.compile("^\\s*(\\d+)\\s+([\\d\\.\\*]+)\\s+(.*)$");
                Matcher matcher = pattern.matcher(line);
                if (matcher.find()) {
                    String ip = matcher.group(2);
                    String rest = matcher.group(3).trim();
                    List<String> times = new ArrayList<>();
                    Matcher timeMatcher = Pattern.compile("(\\d+\\.\\d+ ms|\\*)").matcher(rest);
                    while (timeMatcher.find()) {
                        times.add(timeMatcher.group(1));
                    }
                    hop.put("ip", ip);
                    hop.put("times", times);
                    return hop;
                } else {
                    return null;
                }
            }
        } catch (Exception e) {
            return null;
        }
    }
    public static class TracerouteHop {
        public int hop;
        public String ip;
        public List<Double> times;

        public TracerouteHop(int hop, String ip, List<Double> times) {
            this.hop = hop;
            this.ip = ip;
            this.times = times;
        }
    }

    public static class TracerouteResult {
        public int id;
        public String host;
        public String timestamp;
        public List<TracerouteHop> hops;

        public TracerouteResult(int id, String host, String timestamp, List<TracerouteHop> hops) {
            this.id = id;
            this.host = host;
            this.timestamp = timestamp;
            this.hops = hops;
        }
    }

    @GetMapping
    public List<TracerouteResult> getAllTraceroutes() {
        List<TracerouteResult> results = new ArrayList<>();
        try (Connection conn = database.getInstance().getConnection()) {

            String sqlResults = "SELECT * FROM traceroute_results ORDER BY id DESC";
            try (PreparedStatement stmt = conn.prepareStatement(sqlResults);
                 ResultSet rs = stmt.executeQuery()) {

                while (rs.next()) {
                    int tracerouteId = rs.getInt("id");
                    String host = rs.getString("host");
                    String timestamp = rs.getString("timestamp");


                    List<TracerouteHop> hops = new ArrayList<>();
                    String sqlHops = "SELECT * FROM traceroute_hops WHERE traceroute_id = ? ORDER BY hop_number ASC";
                    try (PreparedStatement stmtHop = conn.prepareStatement(sqlHops)) {
                        stmtHop.setInt(1, tracerouteId);
                        try (ResultSet rsHop = stmtHop.executeQuery()) {
                            while (rsHop.next()) {
                                int hopNumber = rsHop.getInt("hop_number");
                                String ip = rsHop.getString("ip");
                                List<Double> times = new ArrayList<>();
                                for (int i = 1; i <= 3; i++) {
                                    double t = rsHop.getDouble("time" + i + "_ms");
                                    if (!rsHop.wasNull()) times.add(t);
                                    else times.add(null);
                                }
                                hops.add(new TracerouteHop(hopNumber, ip, times));
                            }
                        }
                    }

                    results.add(new TracerouteResult(tracerouteId, host, timestamp, hops));
                }

            }

        } catch (SQLException e) {
            throw new RuntimeException(e);
        }

        return results;
    }



}