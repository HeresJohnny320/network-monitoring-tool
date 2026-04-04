package hj320.network_tool;

import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class timerutil {

    private static final ScheduledExecutorService scheduler = Executors.newScheduledThreadPool(1);

    public static long parseDuration(String input) {
        long totalMs = 0;
        Pattern pattern = Pattern.compile("(\\d+)([hms])");
        Matcher matcher = pattern.matcher(input.toLowerCase());

        while (matcher.find()) {
            int value = Integer.parseInt(matcher.group(1));
            String unit = matcher.group(2);

            switch (unit) {
                case "h" -> totalMs += value * 60L * 60 * 1000;
                case "m" -> totalMs += value * 60L * 1000;
                case "s" -> totalMs += value * 1000L;
            }
        }
        return totalMs;
    }

    public static void scheduleRepeating(String duration, Runnable task) {
        long intervalMs = parseDuration(duration);

        if (intervalMs <= 0) {
            throw new IllegalArgumentException("Duration must be positive!");
        }

        scheduler.scheduleAtFixedRate(
                task,
                0,
                intervalMs,
                TimeUnit.MILLISECONDS
        );
    }
}