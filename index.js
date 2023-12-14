const express = require('express');
const path = require('path');
const directories = {
    'cv4x.net': {
        path: '../cv4x.net',
        port: 8080
    },
    'svstudio-guides': {
        path: '../svstudio-guides',
        port: 8081
    },
    'js-lib': {
        path: '../js-lib',
        port: 8082
    },
    cvjs: {
        path: '../cvjs',
        port: 8083
    }
};

for (let key in directories) {
    const dir = directories[key];
    const app = express();
    app.use(express.static(path.join(__dirname, dir.path)));
    app.listen(dir.port, console.log.bind(null, `Serving ${key} on port ${dir.port}`));
}