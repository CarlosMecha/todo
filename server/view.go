package server

const htmlView = `
<!DOCTYPE html>
<html>
    <head>
        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
        <script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/markdown-it/8.4.0/markdown-it.min.js"></script>
        <title>TODO</title>
    </head>
    <body>
        <form action="#" onsubmit="return get()">
            <input id="auth" type="text" name="auth" value="Auth"/>
            <input type="submit">
        </form>
        <div id="view" style="width: 600px; padding: 0 10px"></div>
        <div id="markdown">{ .Body }</div>
        <script type="text/javascript">
            var markdown = document.getElementById("markdown");
            var view = document.getElementById("view");
            view.innerHTML = (window.markdownit()).render(markdown.text);
			markdown.style.visibility = "hidden";
      </script>
    </body>
</html>
`
