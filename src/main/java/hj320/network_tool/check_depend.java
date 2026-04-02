package hj320.network_tool;

import java.io.BufferedReader;
import java.io.InputStreamReader;

public class check_depend {
    public static boolean isSpeedtestInstalled() {
        try {
            ProcessBuilder pb = new ProcessBuilder("speedtest", "--version");
            pb.redirectErrorStream(true);
            Process process = pb.start();

            BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
            String output = reader.readLine();
            process.waitFor();

            return output != null && !output.isEmpty();

        } catch (Exception e) {
            return false;
        }
    }


    public static boolean isCommandAvailable(String command) {
        try {
            ProcessBuilder pb = new ProcessBuilder(command);
            pb.redirectErrorStream(true);
            Process process = pb.start();

            BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
            String output = reader.readLine();
            process.waitFor();

            return output != null || process.exitValue() == 0;

        } catch (Exception e) {
            return false;
        }
    }
    public static boolean dependisinstalled(){
        String os = System.getProperty("os.name").toLowerCase();
        String tracerouteCmd = os.contains("win") ? "tracert" : "traceroute";
        boolean speed = isSpeedtestInstalled();
        boolean ping = isCommandAvailable("ping");
        boolean trace = isCommandAvailable(tracerouteCmd);
        System.out.println("Speedtest-cli available: " + speed);
        System.out.println("Ping available: " + ping);
        System.out.println("Traceroute available: " + trace);


        boolean everything = false;
        if(speed == true && ping == true && trace == true) {
            everything = true;
        }
        return everything;
    }
}
