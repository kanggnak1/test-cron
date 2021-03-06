local entry_display = require("telescope.pickers.entry_display")
local M = {}

function M.gen_from_run(opts)
  opts = opts or {}
  local name = vim.split(opts.full_name, "/")[2]

  local displayer = entry_display.create({
    separator = " ",
    items = {
      { width = 60 },
      { remaining = true },
    },
  })

  local make_display = function(entry)
    return displayer({
      entry.title,
      entry.category,
    })
  end

  return function(entry)
    local result = {
      ordinal = entry.attributes.title,
      category = entry.attributes.category,
      title = entry.attributes.title,
      slug = entry.attributes.slug,
      display = make_display,
    }

    if entry.attributes.category == "data-sources" or entry.attributes.category == "resources" then
      result.title = name .. "_" .. entry.attributes.title
      result.ordinal = name .. "_" .. entry.attributes.title
    end

    return result
  end
end

function M.gen_from_providers(opts)
  opts = opts or {}

  local displayer = entry_display.create({
    separator = " ",
    items = {
      { width = 20 },
      { remaining = true },
    },
  })

  local make_display = function(entry)
    return displayer({
      entry.name,
      entry.description,
    })
  end
  return function(entry)
    return {
      ordinal = entry.name,
      name = entry.name,
      full_name = entry.full_name,
      description = entry.description,
      display = make_display,
    }
  end
end

return M
