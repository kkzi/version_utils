# 制作安装包及版本检测更新工具

## 制作安装包

根据配置项复制新的编译产物到目标目录，并调用 inno setup 制作安装包

## 版本更新检测

启动一个 http server，检测指定目录下的安装文件，根据请求返回最新版本及下载地址

安装文件命名必须为 Program_1.1.0.exe 格式
检测更新 http 请求地址为 
http://127.0.0.1:9999/check_version/Program

检测结果返回格式为
~~~json
{
    "code":0,   
    "description":"",  
    "content":{  
        "modified_time":"1537354814",  
        "url":"http://127.0.0.1:9999/Program_1.1.0.exe",    
        "version":"2.1.5"  
    }  
}
~~~
