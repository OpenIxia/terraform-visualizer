var gulp = require('gulp');
var mkdirp = require('mkdirp');
var rename = require('gulp-rename')

var spawn = require('child_process').spawn;
var fs = require('fs');

//
// generate hcl wrapper
//
gulp.task('generate-hcl-container', (done) => {
    var docker = spawn('docker', [
        'build',
        '-t', 'gopher-hcl-gopherjs',
        '-f', 'hcl-hil/gopherjs.Dockerfile',
        'hcl-hil'], { stdio: 'inherit' });

    docker.on('close', (code) => {
        if (code !== 0) {
            done(new Error(`docker failed with code ${code}`));
        } else {
            done();
        }
    });
});

gulp.task('generate-non-closure.js', ['generate-hcl-container'], (done) => {
    var docker = spawn('docker', [
        'run',
        '--rm', 'gopher-hcl-gopherjs'
    ], { stdio: ['ignore', 'pipe', 'inherit'] });

    var stream = fs.createWriteStream('out/src/hcl-hil.js', { flags: 'w+' });

    docker.stdout.pipe(stream);
    docker.on('close', (code) => {
        if (code !== 0) {
            done(new Error(`docker run gopher-hcl-gopherjs failed with code ${code}`));
        } else {
            done();
        }
    });
});
gulp.task('generate-transpiled.js', ['generate-hcl-container'], (done) => {
    var docker = spawn('docker', [
        'run',
        '--rm', 'gopher-hcl-gopherjs'
    ], { stdio: ['ignore', 'pipe', 'inherit'] });

    var stream = fs.createWriteStream('hcl-hil/transpiled.js', { flags: 'w+' });

    docker.stdout.pipe(stream);
    docker.on('close', (code) => {
        if (code !== 0) {
            done(new Error(`docker run gopher-hcl-gopherjs failed with code ${code}`));
        } else {
            done();
        }
    });
});

gulp.task('create-output-directory', (done) => {
    mkdirp('out/src', done);
});

gulp.task('generate-closure-container', ['generate-transpiled.js'], (done) => {
    var docker = spawn('docker', [
        'build',
        '-t', 'gopher-hcl-closure-compiler',
        '-f', 'hcl-hil/closure.Dockerfile',
        'hcl-hil'], { stdio: 'inherit' });

    docker.on('close', (code) => {
        if (code !== 0) {
            done(new Error(`docker failed with code ${code}`));
        } else {
            done();
        }
    });
});

gulp.task('generate-hcl-hil.js', ['create-output-directory'], (done) => {

    gulp.src('hcl-hil/transpiled.js')
        .pipe(rename('hcl-hil.js'))
        .pipe(gulp.dest('out/src'))

});

//
// default
//
gulp.task('default', ['generate-hcl-hil.js']);