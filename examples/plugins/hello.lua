-- Example Lua plugin for TermCode.
--
-- Install: copy this file to ~/.config/termcode/plugins/hello.lua
-- (or any directory listed in $TERMCODE_PLUGIN_PATH).
--
-- No rebuild of TermCode required. The plugin is loaded at startup.
--
-- Try it:
--   - In chat, type:  /hello Neko
--   - Press Ctrl+P, then type "hello"
--   - Ask the AI to "use example_echo to say hi"
--   - Look at the brand color — it should be hot pink

-- Tool the AI can call. Signature: (params table) -> (string, err?)
termcode.register_tool(
    "example_echo",
    "Echo the input back to the AI.",
    "text (string, required) — the text to echo",
    function(params)
        local text = params.text or ""
        if text == "" then
            return nil, "missing 'text' parameter"
        end
        return text
    end
)

-- Slash command users can type. (string) -> (string, err?)
termcode.register_command(
    "/hello",
    "Say hi (optionally to a name).",
    function(argv)
        if argv == "" or argv == nil then
            return "Hi from the hello Lua plugin!"
        end
        return "Hi, " .. argv .. "! (from /hello)"
    end
)

-- Palette item. () -> (string, err?)
termcode.register_palette(
    "Hello — say hi",
    "Inserts a friendly greeting from the hello plugin.",
    function()
        return "Hello from the hello palette item!"
    end
)

-- Theme override. Any subset of the palette.
termcode.set_theme({
    primary = "EC4899", -- hot pink instead of lavender
    accent  = "F472B6",
})

-- System-prompt fragment. Appended to the AI's system prompt.
termcode.append_prompt([[
## Hello plugin active
This session has the hello Lua plugin loaded. Keep replies under
3 sentences unless asked otherwise.
]])
