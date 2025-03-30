print("#START SCRIPT");
for (var i = 0; i< workspace.screens.length; i++) {
    var screen = workspace.screens[i]
    var out = "{"
    out += "\"name\": \""+screen.name+"\","
    out += "\"manufacturer\": \""+screen.manufacturer+"\","
    out += "\"model\": \""+screen.model+"\","
    out += "\"serial\": \""+screen.serialNumber+"\","
    out += "\"pixelRatio\": "+screen.devicePixelRatio
    out += "\"geometry\": {"
    out += "\"topLeft\": {"
    out += "\"x\":"+screen.geometry.left+","
    out += "\"y\":"+screen.geometry.top
    out += "},"
    out += "\"bottomRight\": {"
    out += "\"x\":"+screen.geometry.right+","
    out += "\"y\":"+screen.geometry.bottom
    out += "}"
    out += "}"
    out += "}"
    print(out)
}
print("#END SCRIPT");
