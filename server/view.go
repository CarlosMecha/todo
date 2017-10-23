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
        <script type="text/javascript">
        function get(){
            var input = document.getElementById("auth");
            var view = document.getElementById("view");
			var xmlhttp = new XMLHttpRequest();
			var token = input.value
            xmlhttp.onreadystatechange = function(){
                if (xmlhttp.readyState == 4 && xmlhttp.status == 200){
                    view.innerHTML = (window.markdownit()).render(xmlhttp.responseText);
                    document.getElementsByTagName("form")[0].style.visibility = "hidden";
                } else {
                    input.value = "Error requesting file";
                }
            }
            xmlhttp.open("GET", "/", true);
            xmlhttp.setRequestHeader("Token", token);
			xmlhttp.send();
			return false
        }
      </script>
    </body>
</html>
`
