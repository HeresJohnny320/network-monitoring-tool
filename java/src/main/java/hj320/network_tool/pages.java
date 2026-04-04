package hj320.network_tool;

import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;

@Controller
public class pages {

    @GetMapping("/")
    public static String mainpage() {
        return "index.html";
    }
}
