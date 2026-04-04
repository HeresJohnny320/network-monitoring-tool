package hj320.network_tool;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

public class readjson {
    private JsonNode rootNode;
    private final File configFile;
    private final ObjectMapper mapper = new ObjectMapper();

    public readjson() throws IOException {
        configFile = new File("config.json");

        if (!configFile.exists()) {
            ObjectNode defaultConfig = mapper.createObjectNode();
            defaultConfig.put("run_sql", false);
            defaultConfig.put("sql_user", "user");
            defaultConfig.put("sql_password", "password");
            defaultConfig.put("sql_host", "localhost");
            defaultConfig.put("sql_port", "3306");
            defaultConfig.put("sql_database", "my_database");
            defaultConfig.put("run_every", "1h");
            defaultConfig.put("speedtest_server_id", "");
            ArrayNode pingHosts = defaultConfig.putArray("ping_host");
            pingHosts.add("google.com");
            pingHosts.add("github.com");

            ArrayNode tracerouteHosts = defaultConfig.putArray("traceroute_host");
            tracerouteHosts.add("google.com");
            tracerouteHosts.add("github.com");

            mapper.writerWithDefaultPrettyPrinter().writeValue(configFile, defaultConfig);
            System.out.println("Created default config.json");
        }

        rootNode = mapper.readTree(configFile);
    }

    public boolean getBoolean(String key, boolean defaultValue) {
        JsonNode node = rootNode.path(key);
        return node.isMissingNode() ? defaultValue : node.asBoolean(defaultValue);
    }

    public String getString(String key, String defaultValue) {
        JsonNode node = rootNode.path(key);
        return node.isMissingNode() ? defaultValue : node.asText(defaultValue);
    }

    public int getInt(String key, int defaultValue) {
        JsonNode node = rootNode.path(key);
        return node.isMissingNode() ? defaultValue : node.asInt(defaultValue);
    }

    public ArrayNode getArray(String key) {
        JsonNode node = rootNode.path(key);
        return node.isArray() ? (ArrayNode) node : null;
    }

    public List<String> getStringList(String key) {
        ArrayNode array = getArray(key);
        List<String> list = new ArrayList<>();
        if (array != null) {
            for (JsonNode node : array) {
                list.add(node.asText());
            }
        }
        return list;
    }

    public List<Integer> getIntList(String key) {
        ArrayNode array = getArray(key);
        List<Integer> list = new ArrayList<>();
        if (array != null) {
            for (JsonNode node : array) {
                list.add(node.asInt());
            }
        }
        return list;
    }

    public JsonNode getNode(String key) {
        JsonNode node = rootNode.path(key);
        return node.isMissingNode() ? null : node;
    }

    public void save() throws IOException {
        mapper.writerWithDefaultPrettyPrinter().writeValue(configFile, rootNode);
    }

    public JsonNode getRootNode() {
        return rootNode;
    }
}