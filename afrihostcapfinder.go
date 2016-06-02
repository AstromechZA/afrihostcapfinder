package main

import (
    "fmt"
    "flag"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "os"
    "io/ioutil"
    "regexp"
    "strings"
    "strconv"
)

const usageString =
`This binary will connect to Afrihost Clientzone using the provided email address
and password. It will return the quantity of bandwidth left on the accounts
connectivity page in Gigabytes.

If an error occurs, it will print to stderr and exit with code 1.

It requires an email address and a password, the password should be provided
from a file. Best practise is for this file to have 0600 permissions.

`

func mainInner() error {
    // set up command line parsing
    emailFlag := flag.String("email", "", "Email for Afrihost Clientzone")
    passwordFileFlag := flag.String("passwordfile", "", "A file containing the password for Afrihost Clientzone")

    flag.Usage = func() {
        os.Stderr.WriteString(usageString)
        flag.PrintDefaults()
    }

    flag.Parse()

    clientzoneEmail := strings.TrimSpace(*emailFlag)
    if clientzoneEmail == "" {
        return fmt.Errorf("Please provide a non-empty email address via the -email flag")
    }

    clientzonePasswordFile := strings.TrimSpace(*passwordFileFlag)
    if clientzonePasswordFile == "" {
        return fmt.Errorf("Please provide a non-empty file path via the -passwordfile flag")
    }

    // now read password file
    passwordContent, err := ioutil.ReadFile(clientzonePasswordFile)
    if err != nil { return err }
    clientzonePassword := strings.TrimSpace(string(passwordContent))

    // set up the http client object which will also hold the cookie session
    cookieJar, _ := cookiejar.New(nil)
    client := &http.Client{Jar: cookieJar}

    // do original request to get first clientzone cookie
    response, err := client.Get("https://clientzone.afrihost.com/en/")
    if err != nil { return err }

    // now build up POST request params
    postData := url.Values{}
    postData.Set("_username", clientzoneEmail)
    postData.Set("_password", clientzonePassword)

    // and submit it - attempt log in
    response, err = client.PostForm("https://clientzone.afrihost.com/en/login_check", postData)
    if err != nil { return err }

    // a successful log in will redirect the user to the /en/ landing page
    if response.Request.URL.String() != "https://clientzone.afrihost.com/en/" {
        return fmt.Errorf("Incorrect username or password")
    }

    // now fetch the connectivity page
    response, err = client.Get("https://clientzone.afrihost.com/en/my-connectivity")
    if err != nil { return err }

    // make sure we close the body channel even if errors occur
    defer response.Body.Close()

    // read all
    body, _ := ioutil.ReadAll(response.Body)
    bodyText := string(body)

    // now we need to extract the values with a big piece of regex
    r, _ := regexp.Compile("<p>\\s*([\\d\\.]+)\\s*<span class=\"small\">\\s*([A-Za-z]+?)\\s*</span>\\s*<span class=\"descriptor\">\\s*REMAINING\\s*</span>")
    match := r.FindStringSubmatch(bodyText)
    if match == nil { return fmt.Errorf("Could not extract remaining bandwidth from Afrihost page") }

    // pull out the selected fields
    quantity, err := strconv.ParseFloat(match[1], 32)
    if err != nil { return err }

    units := strings.ToLower(match[2])

    if units == "gb" {
        // do nothing
    } else if units == "mb" {
        quantity /= 1000
    } else if units == "tb" {
        quantity *= 1000
    } else {
        return fmt.Errorf("Unknown bandwidth unit '%s'", units)
    }

    fmt.Printf("%.3f\n", quantity)
    return nil
}

func main() {
    if err := mainInner(); err != nil {
        os.Stderr.WriteString(err.Error() + "\n")
        os.Exit(1)
    }
}
