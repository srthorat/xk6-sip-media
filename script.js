import sip from 'k6/x/sip';
import { check } from 'k6';

// This is the default Quickstart script for xk6-sip-media.
// Run it easily using: make run

export const options = {
    vus: 1,
    duration: '10s',
};

export default function () {
    // A simple sanity loopback call against a dummy / echo target.
    // If you have a real PBX/SBC, replace the target below.
    const result = sip.call({
        target: 'sip:echo@127.0.0.1:5060',  
        duration: '5s',
        audioMode: 'silent',  // Send comfort noise to avoid needing a .wav for quickstart
    });

    check(result, {
        'call attempted': (r) => r !== undefined,
    });
}
